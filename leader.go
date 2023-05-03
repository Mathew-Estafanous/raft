package raft

import (
	"github.com/Mathew-Estafanous/raft/pb"
	"sync"
	"time"
)

type leader struct {
	*Raft
	heartbeat     *time.Timer
	appendEntryCh chan appendEntryResp

	indexMu    sync.Mutex
	nextIndex  map[uint64]int64
	matchIndex map[uint64]int64

	tasks map[int64]*logTask
}

func (l *leader) getType() raftState {
	return Leader
}

func (l *leader) runState() {
	l.heartbeat.Reset(l.opts.HeartBeatTimout)
	l.indexMu.Lock()
	for k := range l.cluster.AllNodes() {
		// Initialize own matchIndex with the last index that has been
		// added to own log.
		if k == l.id {
			l.matchIndex[k] = l.log.LastIndex()
		} else {
			l.matchIndex[k] = -1
		}
		l.nextIndex[k] = l.log.LastIndex() + 1
	}
	l.indexMu.Unlock()

	// Before breaking out of leader state, respond to any remaining tasks
	// with the appropriate error.
	defer func() {
		var respErr error
		select {
		case <-l.shutdownCh:
			respErr = ErrRaftShutdown
		default:
			n, err := l.cluster.GetNode(l.leaderId)
			if err != nil {
				l.logger.Fatalf("[BUG] Couldn't find a leader with ID %v", l.leaderId)
			}
			respErr = NewLeaderError(n.ID, n.Addr)
		}

		for _, v := range l.tasks {
			v.respond(respErr)
		}
	}()

	for l.getState().getType() == Leader {
		select {
		case <-l.heartbeat.C:
			for _, n := range l.cluster.AllNodes() {
				if n.ID != l.id {
					go l.sendAppendReq(n, l.nextIndex[n.ID], true)
				}
			}
			l.heartbeat.Reset(l.opts.HeartBeatTimout)
		case ae := <-l.appendEntryCh:
			if ae.error != nil {
				break
			}
			l.handleAppendResp(ae)
		case lt := <-l.applyCh:
			lt.log.Term = l.fromStableStore(keyCurrentTerm)
			lt.log.Index = l.log.LastIndex() + 1

			if err := l.log.AppendLogs([]*Log{lt.log}); err != nil {
				l.logger.Printf("Failed to store new log %v.", lt.log)
				lt.respond(ErrFailedToStore)
				break
			}

			// Add the logTask to a map of all the tasks currently being run.
			l.tasks[lt.log.Index] = lt

			l.indexMu.Lock()
			l.matchIndex[l.id] += 1
			l.indexMu.Unlock()

			// resetting the heartbeat time since we are going to send append entries, which
			// would make a heartbeat unnecessary.
			l.heartbeat.Reset(l.opts.HeartBeatTimout)

			for _, n := range l.cluster.AllNodes() {
				if n.ID != l.id {
					go l.sendAppendReq(n, l.nextIndex[n.ID], false)
				}
			}
		case <-l.snapTimer.C:
			l.onSnapshot()
		case <-l.shutdownCh:
			return
		}
	}
}

func (l *leader) sendAppendReq(n Node, nextIdx int64, isHeartbeat bool) {
	l.indexMu.Lock()
	prevIndex := nextIdx - 1
	var prevTerm uint64
	if prevIndex <= -1 {
		prevTerm = 0
	} else {
		log, err := l.log.GetLog(prevIndex)
		if err != nil {
			l.logger.Printf("Failed to get log %v from log store", prevIndex)
			return
		}
		prevTerm = log.Term
	}

	if !isHeartbeat || nextIdx <= l.log.LastIndex() {
		nextIdx++
	}
	l.indexMu.Unlock()
	logs, err := l.log.AllLogs()
	if err != nil {
		l.logger.Printf("Failed to get all logs from store.")
		return
	}
	// must offset to 0th index of the log slice. Use the index of the 0th log
	// as the base of the offset.
	idxOffset := logs[0].Index
	matchIndex := max(0, l.matchIndex[n.ID]+1-idxOffset)
	nextIndex := max(0, nextIdx-idxOffset)
	logs = logs[matchIndex:nextIndex]

	l.mu.Lock()
	req := &pb.AppendEntriesRequest{
		Term:         l.fromStableStore(keyCurrentTerm),
		LeaderId:     l.id,
		LeaderCommit: l.commitIndex,
		PrevLogIndex: prevIndex,
		PrevLogTerm:  prevTerm,
		Entries:      logsToEntries(logs),
	}
	l.mu.Unlock()

	r := l.sendRPC(req, n)
	l.appendEntryCh <- appendEntryResp{r, n.ID}
}

func (l *leader) handleAppendResp(ae appendEntryResp) {
	r := ae.resp.(*pb.AppendEntriesResponse)
	if r.Term > l.fromStableStore(keyCurrentTerm) {
		l.setState(Follower)
		l.setStableStore(keyCurrentTerm, r.Term)
		l.setStableStore(keyVotedFor, 0)
		return
	}

	l.indexMu.Lock()
	defer l.indexMu.Unlock()
	if r.Success {
		l.matchIndex[ae.nodeId] = l.nextIndex[ae.nodeId] - 1
		l.nextIndex[ae.nodeId] = min(l.log.LastIndex()+1, l.nextIndex[ae.nodeId]+1)
	} else {
		l.nextIndex[ae.nodeId] -= 1
	}
	// Check if a majority of nodes in the raft Cluster have matched their logs
	// at index N. If most have replicated the log then we can consider logs up to
	// index N to be committed.
	for N := l.log.LastIndex(); N > l.commitIndex; N-- {
		if yes := l.majorityMatch(N); yes {
			l.setCommitIndex(N)
			l.mu.Lock()
			// apply the new committed logs to the FSM.
			l.applyLogs()
			l.mu.Unlock()
			break
		}
	}
}

func (l *leader) setCommitIndex(comIdx int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := l.commitIndex + 1; i <= comIdx; i++ {
		t, ok := l.tasks[i]
		if !ok {
			continue
		}
		t.respond(nil)
		delete(l.tasks, i)
	}
	l.logger.Printf("Update Commit Index: From %v -> %v", l.commitIndex, comIdx)
	l.commitIndex = comIdx
}

func (l *leader) majorityMatch(N int64) bool {
	majority := l.cluster.Quorum()
	matchCount := 0
	for _, v := range l.matchIndex {
		if v == N {
			matchCount++
		}

		if matchCount >= majority {
			return true
		}
	}
	return false
}

type appendEntryResp struct {
	rpcResp
	nodeId uint64
}

func min(a, b int64) int64 {
	if a <= b {
		return a
	}
	return b
}

func max(a, b int64) int64 {
	if a >= b {
		return a
	}
	return b
}

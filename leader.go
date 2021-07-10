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
	l.heartbeat.Reset(l.cluster.heartBeatTime)
	l.indexMu.Lock()

	for k := range l.cluster.Nodes {
		// Initialize own matchIndex with the last index that has been
		// added to own log.
		if k == l.id {
			l.matchIndex[k] = l.logStore.LastIndex()
		} else {
			l.matchIndex[k] = -1
		}
		l.nextIndex[k] = l.logStore.LastIndex() + 1
	}
	l.indexMu.Unlock()

	// Before breaking out of leader state, respond to any remaining tasks
	// with an ErrNotLeader error.
	defer func() {
		for _, v := range l.tasks {
			v.respond(ErrNotLeader)
		}
	}()

	for l.getState().getType() == Leader {
		select {
		case <-l.heartbeat.C:
			for _, n := range l.cluster.Nodes {
				if n.ID == l.id {
					continue
				}
				go l.sendAppendReq(n, l.nextIndex[n.ID], true)
			}
			l.heartbeat.Reset(l.cluster.heartBeatTime)
		case ae := <-l.appendEntryCh:
			if ae.error != nil {
				break
			}
			l.handleAppendResp(ae)
		case lt := <-l.applyCh:
			l.mu.Lock()

			lt.log.Term = l.currentTerm
			lt.log.Index = l.logStore.LastIndex() + 1

			if err := l.logStore.AppendLogs([]*Log{lt.log}); err != nil {
				l.logger.Printf("Failed to store new log %v.", lt.log)
				lt.respond(ErrFailedToStore)
				break
			}

			// Add the logTask to a map of all the tasks currently being run.
			l.tasks[lt.log.Index] = lt

			l.indexMu.Lock()
			l.matchIndex[l.id] += 1
			l.indexMu.Unlock()

			l.lastTerm = l.currentTerm
			l.mu.Unlock()

			// resetting the heartbeat time since we are going to send append entries, which
			// would make a heartbeat unnecessary.
			l.heartbeat.Reset(l.cluster.heartBeatTime)

			for _, n := range l.cluster.Nodes {
				if n.ID == l.id {
					continue
				}
				go l.sendAppendReq(n, l.nextIndex[n.ID], false)
			}
		case <-l.shutdownCh:
			return
		}
	}
}

func (l *leader) sendAppendReq(n node, nextIdx int64, isHeartbeat bool) {
	l.indexMu.Lock()
	prevIndex := nextIdx - 1
	var prevTerm uint64
	if prevIndex <= -1 {
		prevTerm = 0
	} else {
		log, err := l.logStore.GetLog(prevIndex)
		if err != nil {
			l.logger.Printf("Failed to get log %v from log store", prevIndex)
			return
		}
		prevTerm = log.Term
	}

	if !isHeartbeat || nextIdx <= l.logStore.LastIndex() {
		nextIdx++
	}
	l.indexMu.Unlock()
	// TODO: Use the matchIndex as the base instead of sending the entire log.
	logs, err := l.logStore.AllLogs()
	if err != nil {
		l.logger.Printf("Failed to get all logs from store.")
		return
	}
	logs = logs[:nextIdx]

	l.mu.Lock()
	req := &pb.AppendEntriesRequest{
		Term:         l.currentTerm,
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
	if r.Term > l.currentTerm {
		l.setState(Follower)
		l.mu.Lock()
		l.currentTerm = r.Term
		l.votedFor = 0
		l.mu.Unlock()
		return
	}

	l.indexMu.Lock()
	defer l.indexMu.Unlock()
	if r.Success {
		l.matchIndex[ae.nodeId] = l.nextIndex[ae.nodeId] - 1
		l.nextIndex[ae.nodeId] = min(l.logStore.LastIndex()+1, l.nextIndex[ae.nodeId]+1)
	} else {
		l.nextIndex[ae.nodeId] -= 1
	}

	// Check if a majority of nodes in the raft Cluster have matched their logs
	// at index N. If most have replicated the log then we can consider logs up to
	// index N to be committed.
	for N := l.logStore.LastIndex(); N > l.commitIndex; N-- {
		if yes := l.majorityMatch(N); yes {
			l.setCommitIndex(N)
			// apply the new committed logs to the FSM.
			l.applyLogs()
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
			l.logger.Printf("Couldn't find a client task with index %v", i)
		}
		t.respond(nil)
		delete(l.tasks, i)
	}
	l.logger.Printf("Update Commit Index: From %v -> %v", l.commitIndex, comIdx)
	l.commitIndex = comIdx
}

func (l *leader) majorityMatch(N int64) bool {
	majority := l.cluster.quorum()
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

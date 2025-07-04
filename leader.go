package raft

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Mathew-Estafanous/raft/cluster"
)

func (r *Raft) runLeaderState() {
	leaderLogger := r.logger.With(slog.Uint64("term", r.fromStableStore(keyCurrentTerm)))

	r.heartbeat.Reset(r.opts.HeartBeatTimout)
	r.indexMu.Lock()
	for k := range r.cluster.AllNodes() {
		// Initialize own matchIndex with the last index that has been
		// added to own log.
		if k == r.id {
			r.matchIndex[k] = r.log.LastIndex()
		} else {
			r.matchIndex[k] = -1
		}
		r.nextIndex[k] = r.log.LastIndex() + 1
	}
	r.indexMu.Unlock()

	// Before breaking out of leader state, respond to any remaining tasks
	// with the appropriate error.
	defer func() {
		var respErr error
		select {
		case <-r.shutdownCh:
			respErr = ErrRaftShutdown
		default:
			r.leaderMu.Lock()
			id := r.leaderId
			r.leaderMu.Unlock()
			n, err := r.cluster.GetNode(id)
			if err != nil {
				leaderLogger.Warn(fmt.Sprintf("Couldn't find a leader with ID %v", id))
			}
			respErr = NewLeaderError(n.ID, n.Addr)
		}

		for _, v := range r.tasks {
			v.respond(respErr)
		}
	}()

	for r.getState() == Leader {
		select {
		case <-r.heartbeat.C:
			for _, n := range r.cluster.AllNodes() {
				if n.ID != r.id {
					go r.sendAppendReq(n, r.nextIndex[n.ID], true)
				}
			}
			r.heartbeat.Reset(r.opts.HeartBeatTimout)
		case ae := <-r.appendEntryCh:
			if ae.error != nil {
				break
			}
			r.handleAppendResp(ae)
		case lt := <-r.applyCh:
			lt.log.Term = r.fromStableStore(keyCurrentTerm)
			lt.log.Index = r.log.LastIndex() + 1

			if err := r.log.AppendLogs([]*Log{lt.log}); err != nil {
				leaderLogger.Warn(fmt.Sprintf("Failed to store new log %v.", lt.log))
				lt.respond(ErrFailedToStore)
				break
			}

			// Add the logTask to a map of all the tasks currently being run.
			r.tasks[lt.log.Index] = lt

			r.indexMu.Lock()
			r.matchIndex[r.id] += 1
			r.indexMu.Unlock()

			// resetting the heartbeat time since we are going to send append entries, which
			// would make a heartbeat unnecessary.
			r.heartbeat.Reset(r.opts.HeartBeatTimout)

			for _, n := range r.cluster.AllNodes() {
				if n.ID != r.id {
					go r.sendAppendReq(n, r.nextIndex[n.ID], false)
				}
			}
		case <-r.snapTimer.C:
			r.onSnapshot()
		case <-r.stateCh:
			break
		case <-r.shutdownCh:
			return
		}
	}
}

func (r *Raft) sendAppendReq(n cluster.Node, nextIdx int64, isHeartbeat bool) {
	r.indexMu.Lock()
	prevIndex := nextIdx - 1
	var prevTerm uint64
	if prevIndex <= -1 {
		prevTerm = 0
	} else {
		log, err := r.log.GetLog(prevIndex)
		if err != nil {
			r.logger.Warn(fmt.Sprintf("Failed to get log %v from log store", prevIndex))
			return
		}
		prevTerm = log.Term
	}

	if !isHeartbeat || nextIdx <= r.log.LastIndex() {
		nextIdx++
	}
	r.indexMu.Unlock()
	logs, err := r.log.AllLogs()
	if err != nil {
		r.logger.Warn("Failed to get all logs from store.")
		return
	}
	var idxOffset int64
	if len(logs) > 0 {
		// must offset to 0th index of the log slice. Use the index of the 0th log
		// as the base of the offset.
		idxOffset = logs[0].Index
	}
	matchIndex := max(0, r.matchIndex[n.ID]+1-idxOffset)
	nextIndex := max(0, nextIdx-idxOffset)
	if matchIndex > nextIndex {
		logs = logs[max(0, nextIndex-1):nextIndex]
	} else {
		logs = logs[matchIndex:nextIndex]
	}

	r.mu.Lock()
	req := &AppendEntriesRequest{
		Term:         r.fromStableStore(keyCurrentTerm),
		LeaderId:     r.id,
		LeaderCommit: r.commitIndex,
		PrevLogIndex: prevIndex,
		PrevLogTerm:  prevTerm,
		Entries:      logsToEntries(logs),
	}
	r.mu.Unlock()

	appendResp, err := r.transport.SendAppendEntries(context.Background(), n, req)
	r.appendEntryCh <- appendEntryResp{
		rpcResponse: rpcResponse[*AppendEntriesResponse]{
			resp:  appendResp,
			error: err,
		},
		nodeId: n.ID,
	}
}

func (r *Raft) handleAppendResp(ae appendEntryResp) {
	if ae.resp.Term > r.fromStableStore(keyCurrentTerm) {
		r.setState(Follower)
		r.setStableStore(keyCurrentTerm, ae.resp.Term)
		r.setStableStore(keyVotedFor, 0)
		return
	}

	r.indexMu.Lock()
	defer r.indexMu.Unlock()
	if ae.resp.Success {
		r.matchIndex[ae.nodeId] = r.nextIndex[ae.nodeId] - 1
		r.nextIndex[ae.nodeId] = min(r.log.LastIndex()+1, r.nextIndex[ae.nodeId]+1)
	} else {
		r.nextIndex[ae.nodeId] -= 1
	}
	// Check if a majority of nodes in the raft Cluster have matched their logs
	// at index N. If most have replicated the log then we can consider logs up to
	// index N to be committed.
	for N := r.log.LastIndex(); N > r.commitIndex; N-- {
		if yes := majorityMatch(N, r.cluster, r.matchIndex); yes {
			r.setCommitIndex(N)
			r.mu.Lock()
			// apply the new committed logs to the FSM.
			r.applyLogs()
			r.mu.Unlock()
			break
		}
	}
}

func (r *Raft) setCommitIndex(comIdx int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := r.commitIndex + 1; i <= comIdx; i++ {
		t, ok := r.tasks[i]
		if !ok {
			continue
		}
		t.respond(nil)
		delete(r.tasks, i)
	}
	r.logger.Info(fmt.Sprintf("Update commit index from %v to %v.", r.commitIndex, comIdx))
	r.commitIndex = comIdx
}

func majorityMatch(N int64, cluster cluster.Cluster, matchIndex map[uint64]int64) bool {
	majority := cluster.Quorum()
	matchCount := 0
	for _, v := range matchIndex {
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
	rpcResponse[*AppendEntriesResponse]
	nodeId uint64
}

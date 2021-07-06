package raft

import (
	"github.com/Mathew-Estafanous/raft/pb"
	"google.golang.org/protobuf/proto"
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
}

func (l *leader) getType() raftState {
	return Leader
}

func (l *leader) runState() {
	l.heartbeat.Reset(l.cluster.heartBeatTime)
	l.indexMu.Lock()
	for k := range l.cluster.Nodes {
		l.matchIndex[k] = -1
		l.nextIndex[k] = l.lastIndex + 1
	}
	l.indexMu.Unlock()

	for l.getState().getType() == Leader {
		select {
		case <-l.heartbeat.C:
			l.sendHeartbeat()
		case ae := <-l.appendEntryCh:
			if ae.error != nil {
				l.logger.Printf("An append entry request has failed: %v", ae.error)
				break
			}
			// TODO: handle append entry responses from followers.
			r := ae.resp.(*pb.AppendEntriesResponse)
			if r.Term > l.currentTerm {
				l.setState(Follower)
				l.mu.Lock()
				l.currentTerm = r.Term
				l.votedFor = 0
				l.mu.Unlock()
				continue
			}

			l.indexMu.Lock()
			if r.Success {
				l.nextIndex[ae.nodeId] = min(l.lastIndex+1, l.nextIndex[ae.nodeId]+1)
				l.matchIndex[ae.nodeId] = l.nextIndex[ae.nodeId] - 1
			} else {
				l.nextIndex[ae.nodeId] -= 1
			}
			l.indexMu.Unlock()
		case lt := <-l.applyCh:
			l.mu.Lock()
			// resetting the heartbeat time since we are going to send append entries, which
			// would make a heartbeat unnecessary.
			l.heartbeat.Reset(l.cluster.heartBeatTime)
			req := pb.AppendEntriesRequest{
				Term:         l.currentTerm,
				LeaderId:     l.id,
				PrevLogIndex: l.lastIndex,
				PrevLogTerm:  l.lastTerm,
				LeaderCommit: l.commitIndex,
			}

			lt.log.Term = l.currentTerm
			lt.log.Index = l.lastIndex + 1

			l.logMu.Lock()
			l.log = append(l.log, lt.log)
			l.logMu.Unlock()

			l.lastTerm = l.currentTerm
			l.lastIndex++
			l.mu.Unlock()

			for _, n := range l.cluster.Nodes {
				if n.ID == l.id {
					continue
				}
				go l.sendAppendReq(n, proto.Clone(&req).(*pb.AppendEntriesRequest), l.nextIndex[n.ID]+1)
			}
		case <-l.shutdownCh:
			return
		}
	}
}

func (l *leader) sendHeartbeat() {
	l.mu.Lock()
	req := &pb.AppendEntriesRequest{
		Term:         l.currentTerm,
		LeaderId:     l.id,
		PrevLogIndex: l.lastIndex,
		PrevLogTerm:  l.lastTerm,
		LeaderCommit: l.commitIndex,
	}
	l.mu.Unlock()

	for _, n := range l.cluster.Nodes {
		if n.ID == l.id {
			continue
		}
		go l.sendAppendReq(n, proto.Clone(req).(*pb.AppendEntriesRequest), l.nextIndex[n.ID])
	}
	l.heartbeat.Reset(l.cluster.heartBeatTime)
}

type appendEntryResp struct {
	rpcResp
	nodeId uint64
}

func (l *leader) sendAppendReq(n node, req *pb.AppendEntriesRequest, nextIdx int64) {
	l.logMu.Lock()
	l.indexMu.Lock()
	// TODO: Use the matchIndex as the base instead of sending the entire log.
	logs := l.log[:nextIdx]
	l.indexMu.Unlock()
	l.logMu.Unlock()
	req.Entries = logsToEntries(logs)

	r := l.sendRPC(req, n)
	l.appendEntryCh <- appendEntryResp{r, n.ID}
}

func min(a, b int64) int64 {
	if a <= b {
		return a
	}
	return b
}

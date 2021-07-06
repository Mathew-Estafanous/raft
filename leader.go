package raft

import (
	"github.com/Mathew-Estafanous/raft/pb"
	"google.golang.org/protobuf/proto"
	"time"
)

type leader struct {
	*Raft
	heartbeat     *time.Timer
	appendEntryCh chan appendEntryResp

	nextIndex  map[uint64]int64
	matchIndex map[uint64]int64
}

func (l *leader) getType() raftState {
	return Leader
}

func (l *leader) runState() {
	l.heartbeat.Reset(l.cluster.heartBeatTime)
	for k := range l.cluster.Nodes {
		l.matchIndex[k] = -1
		l.nextIndex[k] = l.lastIndex + 1
	}

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
			_ = ae.resp.(*pb.AppendEntriesResponse)
		case _ = <-l.applyCh:
			// TODO: Add new entry to own log before then making request to peers.
			// resetting the heartbeat time since we are going to send append entries to
			// all the peers, which would make the heartbeat repetitive.
			l.heartbeat.Reset(l.cluster.heartBeatTime)

			l.mu.Lock()
			req := pb.AppendEntriesRequest{
				Term:         l.currentTerm,
				LeaderId:     l.id,
				PrevLogIndex: l.lastIndex,
				PrevLogTerm:  l.lastTerm,
				LeaderCommit: l.commitIndex,
			}
			l.mu.Unlock()

			for k, no := range l.cluster.Nodes {
				if k == l.id {
					continue
				}
				go func(n node, req *pb.AppendEntriesRequest) {
					l.logMu.Lock()
					logs := l.log[l.nextIndex[n.ID]:]
					l.logMu.Unlock()
					req.Entries = logsToEntries(logs)

					r := l.sendRPC(req, n)
					l.appendEntryCh <- appendEntryResp{r, n.ID}
				}(no, proto.Clone(&req).(*pb.AppendEntriesRequest))
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

	for _, v := range l.cluster.Nodes {
		if v.ID != l.id {
			go func(n node) {
				r := l.sendRPC(req, n)
				l.appendEntryCh <- appendEntryResp{r, n.ID}
			}(v)
		}
	}
	l.heartbeat.Reset(l.cluster.heartBeatTime)
}

type appendEntryResp struct {
	rpcResp
	peerId uint64
}
package raft

import (
	"github.com/Mathew-Estafanous/raft/pb"
	"time"
)

type leader struct {
	*Raft
	heartbeat     *time.Timer
	appendEntryCh chan rpcResp

	nextIndex  []int64
	matchIndex []int64
}

func (l *leader) getType() raftState {
	return Leader
}

func (l *leader) runState() {
	l.heartbeat.Reset(l.cluster.heartBeatTime)
	l.appendEntryCh = make(chan rpcResp, len(l.cluster.Nodes))
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
			// TODO: Execute log tasks sent to apply channel.
		case <-l.shutdownCh:
			return
		}
	}
}

func (l *leader) sendHeartbeat() {
	l.mu.Lock()
	req := &pb.AppendEntriesRequest{
		Term:     l.currentTerm,
		LeaderId: l.id,
	}
	l.mu.Unlock()

	for _, v := range l.cluster.Nodes {
		if v.ID != l.id {
			go func(n node) {
				r := l.sendRPC(req, n)
				l.appendEntryCh <- r
			}(v)
		}
	}
	l.heartbeat.Reset(l.cluster.heartBeatTime)
}

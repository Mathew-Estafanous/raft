package raft

import (
	"github.com/Mathew-Estafanous/raft/pb"
	"time"
)

type leader struct {
	*Raft
	heartbeat     *time.Timer
	appendEntryCh chan rpcResp
}

func (l *leader) getType() StateType {
	return Leader
}

func (l *leader) runState() {
	l.heartbeat.Reset(l.cluster.heartBeatTime)
	l.appendEntryCh = make(chan rpcResp, len(l.cluster.Nodes))
	for l.getState().getType() == Leader {
		select {
		case <-l.heartbeat.C:
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
		case ae := <-l.appendEntryCh:
			if ae.error != nil {
				l.logger.Printf("An append entry request has failed: %v", ae.error)
				break
			}
			aeResp := ae.resp.(*pb.AppendEntriesResponse)
			l.logger.Println(aeResp)
		case <-l.shutdownCh:
			return
		}
	}
}

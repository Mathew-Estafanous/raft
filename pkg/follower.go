package raft

import (
	"fmt"
	"github.com/Mathew-Estafanous/raft/pkg/pb"
)

func (r *Raft) runFollowerState() {
	r.timer.Reset(randElectTime(r.opts.MinElectionTimeout, r.opts.MaxElectionTimout))
	for r.getState() == Follower {
		select {
		case <-r.timer.C:
			r.logger.Println("Timeout event has occurred.")
			r.setState(Candidate)
			r.leaderId = 0
			return
		case t := <-r.applyCh:
			n, err := r.cluster.GetNode(r.leaderId)
			if err != nil {
				t.respond(ErrFailedToStore)
				break
			}

			if !r.opts.ForwardApply {
				t.respond(NewLeaderError(n.ID, n.Addr))
				break
			}

			resp := sendRPC(&pb.ApplyRequest{
				Command: t.log.Cmd,
			}, n, r.opts.TlsConfig)
			if resp.error != nil {
				r.logger.Printf("Failed to forward apply request to leader: %v", resp.error)
				t.respond(fmt.Errorf("couldn't apply request: %v", resp.error))
				break
			}
			applyResp, ok := resp.resp.(*pb.ApplyResponse)
			if !ok {
				r.logger.Println("[BUG] Couldn't assert apply response type.")
				t.respond(ErrFailedToStore)
				break
			}

			switch applyResp.Result {
			case pb.ApplyResult_Committed:
				t.respond(nil)
			case pb.ApplyResult_Failed:
				t.respond(ErrFailedToStore)
			}
		case <-r.snapTimer.C:
			r.onSnapshot()
		case <-r.shutdownCh:
			return
		}
	}
}

package raft

import (
	"context"
	"fmt"

	"github.com/Mathew-Estafanous/raft/pb"
)

func (r *Raft) runFollowerState() {
	r.timer.Reset(randElectTime(r.opts.MinElectionTimeout, r.opts.MaxElectionTimout))
	for r.getState() == Follower {
		select {
		case <-r.timer.C:
			r.setState(Candidate)
			r.leaderMu.Lock()
			r.leaderId = 0
			r.leaderMu.Unlock()
			return
		case t := <-r.applyCh:
			r.leaderMu.Lock()
			n, err := r.cluster.GetNode(r.leaderId)
			r.leaderMu.Unlock()
			if err != nil {
				t.respond(ErrFailedToStore)
				break
			}

			if !r.opts.ForwardApply {
				t.respond(NewLeaderError(n.ID, n.Addr))
				break
			}

			// TODO: replace background context with apply task ctx when added.
			resp := sendRPC(&pb.ApplyRequest{
				Command: t.log.Cmd,
			}, n, context.Background(), r.opts.TlsConfig, r.opts.Dialer)
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
		case <-r.stateCh:
			break
		case <-r.shutdownCh:
			return
		}
	}
}

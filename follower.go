package raft

import (
	"context"
	"fmt"
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

			applyResp, err := r.transport.SendApplyRequest(context.Background(), n, &ApplyRequest{
				Command: t.log.Cmd,
			})
			if err != nil {
				r.logger.Printf("Failed to forward apply request to leader: %v", err)
				t.respond(fmt.Errorf("couldn't apply request: %v", err))
				break
			}

			switch applyResp.Result {
			case Committed:
				t.respond(nil)
			case Failed:
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

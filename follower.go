package raft

import (
	"fmt"
	"github.com/Mathew-Estafanous/raft/pb"
)

func (r *Raft) runFollowerState() {
	r.timer.Reset(r.randElectTime())
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
				r.logger.Println("Couldn't get leader %v: %v", r.leaderId, err)
			}

			resp := sendRPC(&pb.ApplyRequest{
				Command: t.log.Cmd,
			}, n)
			if resp.error != nil {
				r.logger.Printf("Failed to forward apply request to leader: %v", resp.error)
				t.respond(fmt.Errorf("couldn't apply request: %v", resp.error))
				return
			}
			applyResp, ok := resp.resp.(*pb.ApplyResponse)
			if !ok {
				r.logger.Println("[BUG] Couldn't assert apply response type.")
				t.respond(ErrFailedToStore)
				return
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

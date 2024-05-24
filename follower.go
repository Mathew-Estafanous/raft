package raft

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
				r.logger.Println("[BUG] Couldn't find a leader with ID %v.", r.leaderId)
			}
			t.respond(NewLeaderError(n.ID, n.Addr))
		case <-r.snapTimer.C:
			r.onSnapshot()
		case <-r.shutdownCh:
			return
		}
	}
}

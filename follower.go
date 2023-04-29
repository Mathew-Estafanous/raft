package raft

type follower struct {
	*Raft
}

func (f *follower) getType() raftState {
	return Follower
}

func (f *follower) runState() {
	f.timer.Reset(f.randElectTime())
	for f.getState().getType() == Follower {
		select {
		case <-f.timer.C:
			f.logger.Println("Timeout event has occurred.")
			f.setState(Candidate)
			f.leaderId = 0
			return
		case t := <-f.applyCh:
			n, err := f.cluster.GetNode(f.leaderId)
			if err != nil {
				f.logger.Fatalf("[BUG] Couldn't find a leader with ID %v in the static", f.leaderId)
			}
			t.respond(NewLeaderError(n.ID, n.Addr))
		case <-f.snapTimer.C:
			f.onSnapshot()
		case <-f.shutdownCh:
			return
		}
	}
}

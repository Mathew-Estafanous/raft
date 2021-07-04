package raft

type follower struct {
	*Raft
}

func (f *follower) getType() raftState {
	return Follower
}

func (f *follower) runState() {
	f.timer.Reset(f.cluster.randElectTime())
	for f.getState().getType() == Follower {
		select {
		case <-f.timer.C:
			f.logger.Println("Timeout event has occurred.")
			f.setState(Candidate)
			f.leaderId = -1
			return
		case t := <-f.applyCh:
			t.respond(ErrNotLeader)
		case <-f.shutdownCh:
			return
		}
	}
}

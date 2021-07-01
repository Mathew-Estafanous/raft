package raft

type follower struct {
	*Raft
}

func (f *follower) getType() StateType {
	return Follower
}

func (f *follower) runState() {
	f.timer.Reset(f.cluster.randElectTime())
	for f.getState().getType() == Follower {
		select {
		case <-f.timer.C:
			f.logger.Println("Timeout event has occurred.")
			f.setState(Candidate)
			return
		case <-f.shutdownCh:
			return
		}
	}
}
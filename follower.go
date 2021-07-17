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
			t.respond(ErrNotLeader)
		case <-f.snapTimer.C:
			f.snapTimer.Reset(f.opts.SnapshotTimer)
		case <-f.shutdownCh:
			return
		}
	}
}

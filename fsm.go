package raft

// FSM (finite state machine) defines an interface that must be implemented by
// the client to receive commands sent by the raft cluster.
type FSM interface {
	// Apply will be invoked when a log has been successfully committed and
	// should then be applied upon the state of the fsm.
	Apply(data []byte) error
}

func (r *Raft) runFSM() {
	for {
		select {
		case t := <-r.fsmUpdateCh:
			_ = r.fsm.Apply(t.cmd)
		case <-r.shutdownCh:
			r.logger.Println("Shutting down FSM worker.")
			return
		}
	}
}

type fsmUpdate struct {
	cmd []byte
}

// Task represents an operation that has been sent to the raft cluster. Every task
// represents a future operation that returns when all operations have been applied
// to other raft replications.
type Task interface {
	// Error is a blocking operation that will wait until the task has finished
	// before return the result of the task.
	//
	// A non-nil error will be returned if the task failed to be committed.
	Error() error
}

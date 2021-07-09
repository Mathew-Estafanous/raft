package raft

// FSM (finite state machine) defines an interface that must be implemented by
// the client to receive commands sent by the raft Cluster.
type FSM interface {
	// Apply will be invoked when a log has been successfully committed and
	// should then be applied upon the state of the fsm.
	Apply(data []byte) error
}

func (r *Raft) runFSM() {
	for {
		select {
		case t := <-r.fsmUpdateCh:
			err := r.fsm.Apply(t.cmd)
			if err != nil {
				r.logger.Printf("[FSM ERROR] Failed to properly apply cmd %v.", t.cmd)
			}
		case <-r.shutdownCh:
			r.logger.Println("Shutting down FSM worker.")
			return
		}
	}
}

type fsmUpdate struct {
	cmd []byte
}

// Task represents an operation that has been sent to the raft Cluster. Every task
// represents a future operation that returns when all operations have been applied
// to other raft replications.
type Task interface {
	// Error is a blocking operation that will wait until the task has finished
	// before return the result of the task.
	//
	// A non-nil error will be returned if the task failed to be committed.
	Error() error
}

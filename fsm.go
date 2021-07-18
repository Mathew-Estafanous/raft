package raft

// FSM (finite state machine) defines an interface that must be implemented by
// the client to receive commands sent by the raft Cluster.
type FSM interface {
	// Apply will be invoked when a log has been successfully committed and
	// should then be applied upon the state of the fsm.
	Apply(data []byte) error

	// Snapshot will create a byte slice representation of all the required data
	// to represent the current state of the machine.
	Snapshot() ([]byte, error)
}

func (r *Raft) runFSM() {
	for t := range r.fsmCh {
		switch t := t.(type) {
		case *fsmUpdate:
			err := r.fsm.Apply(t.cmd)
			if err != nil {
				r.logger.Printf("[FSM ERROR] Failed to properly apply cmd %v.", t.cmd)
			}
		case *fsmSnapshot:
			state, err := r.fsm.Snapshot()
			t.state = state
			t.respond(err)
		}
	}
}

type fsmUpdate struct {
	cmd []byte
}

func (f *fsmUpdate) Error() error {
	return nil
}

type fsmSnapshot struct {
	errorTask
	state []byte
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

type errorTask struct {
	errCh chan error
	err   error
}

func (l *errorTask) respond(err error) {
	if l.errCh == nil {
		return
	}

	if l.err != nil {
		return
	}

	l.errCh <- err
	close(l.errCh)
}

func (l *errorTask) Error() error {
	// If an error has already been received previously then we can
	// just return that error.
	if l.err != nil {
		return l.err
	}
	l.err = <-l.errCh
	return l.err
}

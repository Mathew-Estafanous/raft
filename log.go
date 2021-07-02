package raft

// Log entries represent commands that alter the state of the FSM.
// These entries are replicated across a majority of raft instances
// before being considered as committed.
type Log struct {
	// Term contains the election term it was added.
	Term uint64

	// Cmd represents the command applied to the FSM.
	Cmd []byte
}

type logTask struct {
	log *Log

	errCh chan error
	err   error
}

func (l *logTask) respond(err error) {
	if l.errCh == nil {
		return
	}

	if l.err != nil {
		return
	}

	l.errCh <- err
	close(l.errCh)
}

func (l *logTask) Error() error {
	// If an error has already been received previously then we can
	// just return that error.
	if l.err != nil {
		return l.err
	}
	l.err = <-l.errCh
	return l.err
}

package raft

import (
	"fmt"
	"github.com/Mathew-Estafanous/raft/pb"
)

// Log entries represent commands that alter the state of the FSM.
// These entries are replicated across a majority of raft instances
// before being considered as committed.
type Log struct {
	// Index represents the index in the list of log entries.
	Index int64

	// Term contains the election term it was added.
	Term uint64

	// Cmd represents the command applied to the FSM.
	Cmd []byte
}

func (l Log) String() string {
	return fmt.Sprintf("{%d, %v}", l.Term, string(l.Cmd))
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

func logsToEntries(logs []*Log) []*pb.Entry {
	entries := make([]*pb.Entry, 0, len(logs))
	for _, l := range logs {
		entries = append(entries, &pb.Entry{
			Term:  l.Term,
			Index: l.Index,
			Data:  l.Cmd,
		})
	}
	return entries
}

func entriesToLogs(entries []*pb.Entry) []*Log {
	logs := make([]*Log, 0, len(entries))
	for _, e := range entries {
		logs = append(logs, &Log{
			Term:  e.Term,
			Index: e.Index,
			Cmd:   e.Data,
		})
	}
	return logs
}

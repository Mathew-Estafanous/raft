package raft

import (
	"fmt"
)

type logType byte

var (
	LogEntry    logType = 'E'
	LogSnapshot logType = 'S'
)

// Log entries represent commands that alter the state of the FSM.
// These entries are replicated across a majority of raft instances
// before being considered as committed.
type Log struct {
	// Type is the kind of log that this represents.
	Type logType

	// Index represents the index in the list of log entries.
	Index int64

	// Term contains the election term it was added.
	Term uint64

	// Cmd represents the command applied to the FSM.
	Cmd []byte
}

func (l Log) String() string {
	return fmt.Sprintf("{%v}", string(l.Cmd))
}

type logTask struct {
	errorTask
	log *Log
}

func logsToEntries(logs []*Log) []*Entry {
	entries := make([]*Entry, 0, len(logs))
	for _, l := range logs {
		entries = append(entries, &Entry{
			Type:  []byte{byte(l.Type)},
			Term:  l.Term,
			Index: l.Index,
			Data:  l.Cmd,
		})
	}
	return entries
}

func entriesToLogs(entries []*Entry) []*Log {
	logs := make([]*Log, 0, len(entries))
	for _, e := range entries {
		logs = append(logs, &Log{
			Type:  logType(e.Type[0]),
			Term:  e.Term,
			Index: e.Index,
			Cmd:   e.Data,
		})
	}
	return logs
}

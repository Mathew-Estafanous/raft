package raft

import (
	"errors"
)

var (
	ErrLogNotFound   = errors.New("the log could not be found in the storage")
	ErrFailedToStore = errors.New("new log failed to properly be stored")
)

// LogStore defines how a raft's log persistence is handled and the
// required operations for log replication to be successful.
type LogStore interface {
	// LastIndex will return the index of the last log entry that has
	// been added to the log storage.
	LastIndex() int64

	// LastTerm will return the last log term found in the list of log entries.
	LastTerm() uint64

	// GetLog will return the log found at the given index. An error will
	// be returned if the index exceeds the maximum index in the log.
	//
	// If the index is less than the minimum index, than the log at minimum
	// index will be returned instead.
	GetLog(index int64) (*Log, error)

	// AllLogs retrieves every log entry in the store and returns the result.
	AllLogs() ([]*Log, error)

	// AppendLogs will add the slice of logs to the current list of log entries.
	AppendLogs(logs []*Log) error

	// DeleteRange will remove all log entries starting from the min index all
	// the way to the max index (inclusive).
	DeleteRange(min, max int64) error
}

// StableStore is used to provide persistence to vital information related
// to the raft's state.
type StableStore interface {
	Set(key, value []byte) error

	// Get returns the value related to that key. An empty slice is returned if
	// there is no value with that key found. An error signals an invalid store state
	// and will cause the node to terminate to prevent further corruption.
	Get(key []byte) ([]byte, error)
}

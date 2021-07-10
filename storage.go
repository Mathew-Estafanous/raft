package raft

import (
	"errors"
	"sync"
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

	// GetLog will return the log found at the given index. An error will
	// be returned if the index is out of bounds.
	GetLog(index int64) (*Log, error)

	// AllLogs retrieves every log entry in the store and returns the result.
	AllLogs() ([]*Log, error)

	// AppendLogs will add the slice of logs to the current list of log entries.
	AppendLogs(logs []*Log) error

	// DeleteRange will remove all log entries starting from the min index all
	// the way to the max index (inclusive).
	DeleteRange(min, max int64) error
}

type InMemLogStore struct {
	mu   sync.Mutex
	logs []*Log
}

func NewMemLogStore() *InMemLogStore {
	return &InMemLogStore{
		logs: make([]*Log, 0),
	}
}

func (m *InMemLogStore) LastIndex() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return int64(len(m.logs) - 1)
}

func (m *InMemLogStore) GetLog(index int64) (*Log, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if i := m.LastIndex(); i < index {
		return nil, ErrLogNotFound
	}
	return m.logs[index], nil
}

func (m *InMemLogStore) AppendLogs(logs []*Log) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, logs...)
	return nil
}

func (m *InMemLogStore) DeleteRange(min, max int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs[:min], m.logs[max+1:]...)
	return nil
}

func (m *InMemLogStore) AllLogs() ([]*Log, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.logs, nil
}

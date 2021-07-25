package raft

import (
	"errors"
	"fmt"
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
	// there is no value with that key found.
	Get(key []byte) ([]byte, error)
}

// InMemStore is an implementation of the StableStore and LogStore interface.
// Since it is in-memory, all data is lost on shutdown.
//
// NOTE: This implementation is meant for testing and example use-cases and is NOT meant
// to be used in any production environment. It is up to the user to create the wanted
// persistence implementation.
type InMemStore struct {
	mu       sync.Mutex
	logs     []*Log
	lastIdx  int64
	lastTerm uint64

	kvMu sync.Mutex
	kv   map[string][]byte
}

func NewMemStore() *InMemStore {
	return &InMemStore{
		logs:     make([]*Log, 0),
		lastIdx:  -1,
		lastTerm: 0,
		kv:       make(map[string][]byte),
	}
}

func (m *InMemStore) LastIndex() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastIdx
}

func (m *InMemStore) LastTerm() uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastTerm
}

func (m *InMemStore) GetLog(index int64) (*Log, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lastIdx < index {
		return nil, ErrLogNotFound
	}

	minIdx := m.logs[0].Index
	if index < minIdx {
		return m.logs[0], nil
	}
	return m.logs[index-minIdx], nil
}

func (m *InMemStore) AppendLogs(logs []*Log) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, logs...)
	m.updateLastLog()
	return nil
}

func (m *InMemStore) DeleteRange(min, max int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	minIdx := m.logs[0].Index
	if min < minIdx {
		return fmt.Errorf("min %v cannot be less than the current minimum index %v", min, minIdx)
	}
	min -= minIdx
	max -= minIdx
	m.logs = append(m.logs[:min], m.logs[max+1:]...)
	m.updateLastLog()
	return nil
}

func (m *InMemStore) AllLogs() ([]*Log, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	copyLog := make([]*Log, len(m.logs))
	copy(copyLog, m.logs)
	return copyLog, nil
}

func (m *InMemStore) updateLastLog() {
	if len(m.logs)-1 < 0 {
		return
	}

	m.lastIdx = m.logs[len(m.logs)-1].Index
	m.lastTerm = m.logs[len(m.logs)-1].Term
}

func (m *InMemStore) Set(key, value []byte) error {
	m.kvMu.Lock()
	defer m.kvMu.Unlock()
	m.kv[string(key)] = value
	return nil
}

func (m *InMemStore) Get(key []byte) ([]byte, error) {
	m.kvMu.Lock()
	defer m.kvMu.Unlock()
	return m.kv[string(key)], nil
}

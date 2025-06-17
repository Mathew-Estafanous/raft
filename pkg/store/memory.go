package store

import (
	"fmt"
	raft "github.com/Mathew-Estafanous/raft/pkg"
	"sync"
)

// InMemStore is an implementation of the StableStore and LogStore interface.
// Since it is in-memory, all data is lost on shutdown.
//
// NOTE: This implementation is meant for testing and example use-cases and is NOT meant
// to be used in any production environment. It is up to the client to create the persistence store.
type InMemStore struct {
	mu       sync.Mutex
	logs     []*raft.Log
	lastIdx  int64
	lastTerm uint64

	kvMu sync.Mutex
	kv   map[string][]byte
}

func NewMemStore() *InMemStore {
	return &InMemStore{
		logs:     make([]*raft.Log, 0),
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

func (m *InMemStore) GetLog(index int64) (*raft.Log, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lastIdx < index {
		return nil, raft.ErrLogNotFound
	}

	minIdx := m.logs[0].Index
	if index < minIdx {
		return m.logs[0], nil
	}
	return m.logs[index-minIdx], nil
}

func (m *InMemStore) AppendLogs(logs []*raft.Log) error {
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

func (m *InMemStore) AllLogs() ([]*raft.Log, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	copyLog := make([]*raft.Log, len(m.logs))
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

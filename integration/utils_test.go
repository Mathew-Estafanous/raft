package integration

import (
	"github.com/Mathew-Estafanous/raft"
	"sync"
	"time"
)

// testFSM is a simple in-memory key-value store that implements the raft.FSM interface
type testFSM struct {
	mu    sync.RWMutex
	store map[string][]byte
}

func newTestFSM() *testFSM {
	return &testFSM{
		store: make(map[string][]byte),
	}
}

func (f *testFSM) Apply(_ []byte) error {
	// Simple implementation - not needed for leader election tests
	return nil
}

func (f *testFSM) Snapshot() ([]byte, error) {
	// Simple implementation - not needed for leader election tests
	return nil, nil
}

func (f *testFSM) Restore(_ []byte) error {
	// Simple implementation - not needed for leader election tests
	return nil
}

var testOpts = raft.Options{
	MinElectionTimeout: 1 * time.Second,
	MaxElectionTimout:  2 * time.Second,
	HeartBeatTimout:    500 * time.Millisecond,
	SnapshotTimer:      8 * time.Second,
	LogThreshold:       5,
	ForwardApply:       false,
}

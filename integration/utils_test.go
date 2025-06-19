package integration

import (
	"fmt"
	"os"
	"testing"

	"github.com/Mathew-Estafanous/raft"
	"github.com/Mathew-Estafanous/raft/cluster"
	"github.com/Mathew-Estafanous/raft/store"
	"github.com/stretchr/testify/require"

	"sync"
	"time"
)

var testOpts = raft.Options{
	MinElectionTimeout: 1 * time.Second,
	MaxElectionTimout:  2 * time.Second,
	HeartBeatTimout:    500 * time.Millisecond,
	SnapshotTimer:      8 * time.Second,
	LogThreshold:       5,
	ForwardApply:       false,
}

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

type clusterOptFunc func(node cluster.Node, logStore raft.LogStore, stableStore raft.StableStore, raftOptions *raft.Options)

// setupCluster creates a cluster of n Raft nodes for testing
func setupCluster(t *testing.T, n int, opts ...clusterOptFunc) ([]*raft.Raft, func()) {
	t.Helper()

	// Create a test staticCluster configuration
	configFile, err := os.OpenFile(fmt.Sprintf("testdata/cluster_%d.json", n), os.O_RDONLY, 0644)
	require.NoError(t, err)
	defer configFile.Close()

	staticCluster, err := cluster.NewClusterWithConfig(configFile)

	require.NoError(t, err)

	rafts := make([]*raft.Raft, 0, n)

	// Then, create and start all Raft instances
	for _, node := range staticCluster.Nodes {
		fsm := newTestFSM()
		memStore := store.NewMemStore()

		// Apply any additional options to the raft
		for _, opt := range opts {
			opt(node, memStore, memStore, &testOpts)
		}

		raftInst, err := raft.New(staticCluster, node.ID, testOpts, fsm, memStore, memStore)
		require.NoError(t, err)

		rafts = append(rafts, raftInst)
	}

	startClusterFunc := func() {
		for _, r := range rafts {
			go func(raft *raft.Raft) {
				n := staticCluster.AllNodes()[raft.ID()]
				err := raft.ListenAndServe(n.Addr)
				if err != nil {
					t.Logf("Node %d stopped with error: %v", n.ID, err)
				}
			}(r)
		}
	}

	return rafts, startClusterFunc
}

// cleanupTestCluster shuts down all nodes and cleans up resources
func cleanupTestCluster(t *testing.T, rafts []*raft.Raft) {
	t.Helper()

	wg := sync.WaitGroup{}
	wg.Add(len(rafts))

	for _, r := range rafts {
		go func(raft *raft.Raft) {
			defer wg.Done()
			raft.Shutdown()
		}(r)
	}
	wg.Wait()
}

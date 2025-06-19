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
	mu          sync.RWMutex
	store       map[string][]byte
	appliedCmds [][]byte
}

func newTestFSM() *testFSM {
	return &testFSM{
		store:       make(map[string][]byte),
		appliedCmds: make([][]byte, 0),
	}
}

func (f *testFSM) Apply(cmd []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if string(cmd) == "test" {
		// ignore 'test' apply commands as those are used by the testing utils
		// to confirm if a node is a leader (or not)
		return nil
	}

	// Store the command in the applied commands list
	f.appliedCmds = append(f.appliedCmds, cmd)
	return nil
}

func (f *testFSM) Snapshot() ([]byte, error) {
	// Simple implementation - not needed for log replication tests
	return nil, nil
}

func (f *testFSM) Restore(_ []byte) error {
	// Simple implementation - not needed for log replication tests
	return nil
}

type testNode struct {
	id          uint64
	raft        *raft.Raft
	fsm         *testFSM
	logStore    raft.LogStore
	stableStore raft.StableStore
}

type clusterOptFunc func(node cluster.Node, logStore raft.LogStore, stableStore raft.StableStore, raftOptions *raft.Options)

// setupCluster creates a cluster of n Raft nodes for testing
func setupCluster(t *testing.T, n int, opts ...clusterOptFunc) ([]*testNode, func()) {
	t.Helper()

	// Create a test staticCluster configuration
	configFile, err := os.OpenFile(fmt.Sprintf("testdata/cluster_%d.json", n), os.O_RDONLY, 0644)
	require.NoError(t, err)
	defer configFile.Close()

	staticCluster, err := cluster.NewClusterWithConfig(configFile)

	require.NoError(t, err)

	nodes := make([]*testNode, 0, n)

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

		nodes = append(nodes, &testNode{
			id:          node.ID,
			raft:        raftInst,
			fsm:         fsm,
			logStore:    memStore,
			stableStore: memStore,
		})
	}

	startClusterFunc := func() {
		for _, r := range nodes {
			go func(raft *raft.Raft) {
				n := staticCluster.AllNodes()[raft.ID()]
				err := raft.ListenAndServe(n.Addr)
				if err != nil {
					t.Logf("Node %d stopped with error: %v", n.ID, err)
				}
			}(r.raft)
		}
	}

	return nodes, startClusterFunc
}

// cleanupTestCluster shuts down all nodes and cleans up resources
func cleanupTestCluster(t *testing.T, nodes []*testNode) {
	t.Helper()

	wg := sync.WaitGroup{}
	wg.Add(len(nodes))

	for _, n := range nodes {
		go func(raft *raft.Raft) {
			defer wg.Done()
			raft.Shutdown()
		}(n.raft)
	}
	wg.Wait()
}

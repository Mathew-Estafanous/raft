package integration

import (
	"fmt"
	"os"
	"testing"

	"github.com/Mathew-Estafanous/raft"
	"github.com/Mathew-Estafanous/raft/cluster"
	"github.com/Mathew-Estafanous/raft/store"
	"github.com/Mathew-Estafanous/raft/transport"
	"github.com/stretchr/testify/require"

	"sync"
	"time"
)

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
	addr        string
	raft        *raft.Raft
	fsm         *testFSM
	logStore    raft.LogStore
	stableStore raft.StableStore
	options     *raft.Options
	transport   raft.Transport
}

type clusterOptFunc func(node *testNode)

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

	memRegistry := transport.NewRegistry()

	// Then, create and start all Raft instances
	for _, node := range staticCluster.Nodes {
		fsm := newTestFSM()
		memStore := store.NewMemStore()

		testOpts := raft.Options{
			MinElectionTimeout: 2 * time.Second,
			MaxElectionTimout:  4 * time.Second,
			HeartBeatTimout:    500 * time.Millisecond,
			SnapshotTimer:      8 * time.Second,
			LogThreshold:       0,
			ForwardApply:       false,
		}

		tNode := &testNode{
			id:          node.ID,
			addr:        node.Addr,
			fsm:         fsm,
			logStore:    memStore,
			stableStore: memStore,
			options:     &testOpts,
		}

		// Apply any additional options to the raft
		for _, opt := range opts {
			opt(tNode)
		}

		if tNode.transport == nil {
			tNode.transport = transport.NewMemoryTransport(node.Addr, memRegistry)
		}

		raftInst, err := raft.New(staticCluster, node.ID, testOpts, fsm, memStore, memStore, tNode.transport)
		require.NoError(t, err)

		tNode.raft = raftInst
		nodes = append(nodes, tNode)
	}

	startClusterFunc := func() {
		for _, n := range nodes {
			go func(raft *raft.Raft) {
				if err := raft.Serve(); err != nil {
					t.Logf("Node %d stopped with error: %v", n.id, err)
				}
			}(n.raft)
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

// waitForLeader waits for a leader to be elected in the cluster
func waitForLeader(t *testing.T, nodes []*testNode, timeout time.Duration) *raft.Raft {
	t.Helper()

	var leader *raft.Raft
	require.Eventually(t, func() bool {
		for _, n := range nodes {
			if isLeader(n.raft) {
				leader = n.raft
				return true
			}
		}
		return false
	}, timeout, 100*time.Millisecond, "no leader elected within timeout")
	return leader
}

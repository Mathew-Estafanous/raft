package integration

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	raft "github.com/Mathew-Estafanous/raft/pkg"
	"github.com/Mathew-Estafanous/raft/pkg/cluster"
	"github.com/Mathew-Estafanous/raft/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupCluster creates a cluster of n Raft nodes for testing
func setupCluster(t *testing.T, n int) ([]*raft.Raft, *cluster.StaticCluster) {
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

		raftInst, err := raft.New(staticCluster, node.ID, testOpts, fsm, memStore, memStore)
		require.NoError(t, err)

		rafts = append(rafts, raftInst)

		// Start the Raft server in a goroutine
		go func(r *raft.Raft, n cluster.Node) {
			err := r.ListenAndServe(n.Addr)
			if err != nil {
				t.Logf("Node %d stopped with error: %v", n.ID, err)
			}
		}(raftInst, node)
	}

	return rafts, staticCluster
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

// isLeader checks if a node is the leader by attempting to apply a command
// Only the leader can successfully apply commands without returning a LeaderError
func isLeader(r *raft.Raft) bool {
	task := r.Apply([]byte("test"))
	err := task.Error()
	return err == nil
}

// waitForLeader waits for a leader to be elected in the cluster
func waitForLeader(t *testing.T, rafts []*raft.Raft, timeout time.Duration) (*raft.Raft, error) {
	t.Helper()

	timer := time.NewTimer(timeout)
	leaderCh := make(chan *raft.Raft)
	defer func() {
		timer.Stop()
		close(leaderCh)
	}()

	go func() {
		for {
			for _, r := range rafts {
				if isLeader(r) {
					select {
					case leaderCh <- r:
						return
					default:
						// Channel is closed, exit goroutine
						return
					}
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	select {
	case <-timer.C:
		return nil, fmt.Errorf("no leader elected within timeout")
	case leader := <-leaderCh:
		return leader, nil
	}
}

// countLeaders counts the number of leaders in the cluster
func countLeaders(rafts []*raft.Raft) int {
	count := 0
	for _, r := range rafts {
		if isLeader(r) {
			count++
		}
	}
	return count
}

// TestLeaderElectionBasic tests basic leader election functionality
func TestLeaderElectionBasic(t *testing.T) {
	// Create a small cluster with 3 nodes
	rafts, _ := setupCluster(t, 10)
	defer func() {
		cleanupTestCluster(t, rafts)
	}()

	// Wait for a leader to be elected
	leader, err := waitForLeader(t, rafts, 10*time.Second)
	require.NoError(t, err, "Failed to elect a leader")

	t.Logf("Leader elected: Node %d", leader.ID())

	// Verify that exactly one leader was elected
	leaderCount := countLeaders(rafts)
	assert.Equal(t, 1, leaderCount, "Expected exactly one leader")
}

func TestLeaderElection_AfterLeaderFails(t *testing.T) {
	// Create a small cluster with 3 nodes
	rafts, _ := setupCluster(t, 10)
	defer func() {
		cleanupTestCluster(t, rafts)
	}()

	leader, err := waitForLeader(t, rafts, 10*time.Second)
	require.NoError(t, err, "Failed to elect a leader")

	t.Logf("First leader elected: Node %d", leader.ID())

	t.Logf("Shutting down leader %d", leader.ID())
	leader.Shutdown()

	// Wait for a new leader to be elected
	newLeader, err := waitForLeader(t, rafts, 10*time.Second)
	require.NoError(t, err, "Failed to elect a new leader")

	t.Logf("New leader elected: Node %d", newLeader.ID())

	// Verify that exactly one leader was elected
	leaderCount := countLeaders(rafts)
	assert.Equal(t, 1, leaderCount, "Expected exactly one leader")
}

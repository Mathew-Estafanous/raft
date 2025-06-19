package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/Mathew-Estafanous/raft"
	"github.com/Mathew-Estafanous/raft/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// isLeader checks if a node is the leader by attempting to apply a command
// Only the leader can successfully apply commands without returning a LeaderError
func isLeader(r *raft.Raft) bool {
	task := r.Apply([]byte("test"))
	err := task.Error()
	return err == nil
}

// waitForLeader waits for a leader to be elected in the cluster
func waitForLeader(t *testing.T, nodes []*testNode, timeout time.Duration) (*raft.Raft, error) {
	t.Helper()

	timer := time.NewTimer(timeout)
	leaderCh := make(chan *raft.Raft)
	defer func() {
		timer.Stop()
		close(leaderCh)
	}()

	go func() {
		for {
			for _, n := range nodes {
				if isLeader(n.raft) {
					select {
					case leaderCh <- n.raft:
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
func countLeaders(nodes []*testNode) int {
	count := 0
	for _, n := range nodes {
		if isLeader(n.raft) {
			count++
		}
	}
	return count
}

// TestLeaderElectionBasic tests basic leader election functionality
func TestLeaderElectionBasic(t *testing.T) {
	// Create a small cluster with 3 nodes
	nodes, startCluster := setupCluster(t, 10)
	defer func() {
		cleanupTestCluster(t, nodes)
	}()

	startCluster()

	// Wait for a leader to be elected
	leader, err := waitForLeader(t, nodes, 10*time.Second)
	require.NoError(t, err, "Failed to elect a leader")

	t.Logf("Leader elected: Node %d", leader.ID())

	// Verify that exactly one leader was elected
	leaderCount := countLeaders(nodes)
	assert.Equal(t, 1, leaderCount, "Expected exactly one leader")
}

func TestLeaderElection_AfterLeaderFails(t *testing.T) {
	// Create a small cluster with 3 nodes
	rafts, startCluster := setupCluster(t, 10)
	defer func() {
		cleanupTestCluster(t, rafts)
	}()

	startCluster()

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

func TestLeaderElection_OnlyNodesWithLatestLog(t *testing.T) {
	populateLogs := func(n cluster.Node, logStore raft.LogStore, stableStore raft.StableStore, _ *raft.Options) {
		logs := []*raft.Log{
			{
				Type:  raft.Entry,
				Index: 1,
				Term:  1,
				Cmd:   []byte("cmd1"),
			},
			{
				Type:  raft.Entry,
				Index: 2,
				Term:  2,
				Cmd:   []byte("cmd2"),
			},
		}

		if n.ID == 3 {
			return
		}

		t.Logf("Populating logs for node %d", n.ID)
		err := logStore.AppendLogs(logs)
		require.NoError(t, err, "Failed to append logs to log store")
		err = stableStore.Set([]byte("currentTerm"), []byte(fmt.Sprintf("%d", 2)))
		require.NoError(t, err, "Failed to set currentTerm in stable store")
		err = stableStore.Set([]byte("lastApplied"), []byte(fmt.Sprintf("%d", 2)))
		require.NoError(t, err, "Failed to set lastApplied in stable store")
	}

	fasterNodeThree := func(n cluster.Node, _ raft.LogStore, _ raft.StableStore, options *raft.Options) {
		if n.ID != 3 {
			return
		}

		t.Logf("Ensuring node 3 attempts a leader election before other nodes.")
		options.MinElectionTimeout = 700 * time.Millisecond
		options.MaxElectionTimout = 1300 * time.Millisecond
	}

	rafts, startCluster := setupCluster(t, 3, populateLogs, fasterNodeThree)
	defer func() {
		cleanupTestCluster(t, rafts)
	}()

	startCluster()

	leader, err := waitForLeader(t, rafts, 10*time.Second)
	require.NoError(t, err, "Failed to elect a leader")

	require.NotEqual(t, 3, leader.ID(), "Expected a leader other than node 3")
}

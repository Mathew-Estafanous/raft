package integration

import (
	"bytes"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/Mathew-Estafanous/raft"
	"github.com/stretchr/testify/require"
)

func getFollower(t *testing.T, nodes []*testNode) *testNode {
	leader := waitForLeader(t, nodes, 5*time.Second)

	i := slices.IndexFunc(nodes, func(n *testNode) bool {
		return n.id != leader.ID()
	})
	return nodes[i]
}

// TestLogReplication_FromLeader tests that logs are properly replicated from the leader to all followers
func TestLogReplication_FromLeader(t *testing.T) {
	t.Parallel()

	nodes, startCluster := setupCluster(t, 3)
	defer func() {
		cleanupTestCluster(t, nodes)
	}()

	startCluster()

	// Wait for a leader to be elected
	leader := waitForLeader(t, nodes, 10*time.Second)

	t.Logf("Leader elected: Node %d", leader.ID())

	// Apply a command to the leader
	cmd1 := []byte("command1")
	task := leader.Apply(cmd1)
	require.NoError(t, task.Error(), "Failed to apply command to leader")

	// Apply another command to the leader
	cmd2 := []byte("command2")
	task = leader.Apply(cmd2)
	require.NoError(t, task.Error(), "Failed to apply command to leader")

	// Verify that all nodes have added the two commands
	for _, n := range nodes {
		logs, err := n.logStore.AllLogs()
		require.NoError(t, err, "Failed to get logs from node %d", n.id)

		slices.ContainsFunc(logs, func(log *raft.Log) bool {
			return bytes.Equal(log.Cmd, cmd1)
		})

		slices.ContainsFunc(logs, func(log *raft.Log) bool {
			return bytes.Equal(log.Cmd, cmd2)
		})
	}
}

func TestLogReplication_FailsFromFollower_DisabledForwardApply(t *testing.T) {
	t.Parallel()

	nodes, startCluster := setupCluster(t, 3)
	defer func() {
		cleanupTestCluster(t, nodes)
	}()

	startCluster()

	follower := getFollower(t, nodes)

	t.Logf("Sending apply request from follower %d", follower.id)

	cmd := []byte("command1")
	task := follower.raft.Apply(cmd)
	require.Error(t, task.Error(), "Expected apply to fail from follower")
}

func TestLogReplication_FromFollower_EnabledForwardApply(t *testing.T) {
	t.Parallel()

	nodes, startCluster := setupCluster(t, 3, func(node *testNode) {
		node.options.ForwardApply = true
	})
	defer func() {
		cleanupTestCluster(t, nodes)
	}()

	startCluster()

	follower := getFollower(t, nodes)

	t.Logf("Sending apply request from follower %d", follower.id)

	cmd := []byte("command1")
	task := follower.raft.Apply(cmd)
	require.NoError(t, task.Error(), "Expected apply to succeed from follower")

	// Verify that the command was replicated to all nodes
	for _, n := range nodes {
		logs, err := n.logStore.AllLogs()
		require.NoError(t, err, "Failed to get logs from node %d", n.id)

		slices.ContainsFunc(logs, func(log *raft.Log) bool {
			return bytes.Equal(log.Cmd, cmd)
		})
	}
}

func TestLogReplication_LeaderLogsReplicated(t *testing.T) {
	t.Parallel()

	populateLogs := func(node *testNode) {
		logs := []*raft.Log{
			{
				Type:  raft.LogEntry,
				Index: 1,
				Term:  1,
				Cmd:   []byte("cmd1"),
			},
			{
				Type:  raft.LogEntry,
				Index: 2,
				Term:  2,
				Cmd:   []byte("cmd2"),
			},
		}

		if node.id == 3 {
			return
		}

		t.Logf("Populating logs for node %d", node.id)
		err := node.logStore.AppendLogs(logs)
		require.NoError(t, err, "Failed to append logs to log store")
		err = node.stableStore.Set([]byte("currentTerm"), []byte(fmt.Sprintf("%d", 2)))
		require.NoError(t, err, "Failed to set currentTerm in stable store")
		err = node.stableStore.Set([]byte("lastApplied"), []byte(fmt.Sprintf("%d", 2)))
		require.NoError(t, err, "Failed to set lastApplied in stable store")
	}

	nodes, startCluster := setupCluster(t, 3, populateLogs)
	defer func() {
		cleanupTestCluster(t, nodes)
	}()

	startCluster()

	leader := waitForLeader(t, nodes, 10*time.Second)

	t.Logf("Checking that logs from leader %d have replicated to behind node 3", leader.ID())

	i := slices.IndexFunc(nodes, func(n *testNode) bool {
		return n.id == 3
	})
	node := nodes[i]

	logCmd1, err := node.logStore.GetLog(1)
	require.NoError(t, err)
	require.Equal(t, "cmd1", string(logCmd1.Cmd))
	logCmd2, err := node.logStore.GetLog(2)
	require.NoError(t, err)
	require.Equal(t, "cmd2", string(logCmd2.Cmd))
}

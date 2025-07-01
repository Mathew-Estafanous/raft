package integration

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/Mathew-Estafanous/raft"
	"github.com/Mathew-Estafanous/raft/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/benchmark/latency"
)

// isLeader checks if a node is the leader by attempting to apply a command
// Only the leader can successfully apply commands without returning a LeaderError
func isLeader(r *raft.Raft) bool {
	task := r.Apply([]byte("test"))
	err := task.Error()
	return err == nil
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
	t.Parallel()

	nodes, startCluster := setupCluster(t, 10)
	defer func() {
		cleanupTestCluster(t, nodes)
	}()

	startCluster()

	// Wait for a leader to be elected
	leader := waitForLeader(t, nodes, 10*time.Second)

	t.Logf("Leader elected: Node %d", leader.ID())

	// Verify that exactly one leader was elected
	leaderCount := countLeaders(nodes)
	assert.Equal(t, 1, leaderCount, "Expected exactly one leader")
}

func TestLeaderElection_AfterLeaderFails(t *testing.T) {
	t.Parallel()

	rafts, startCluster := setupCluster(t, 10)
	defer func() {
		cleanupTestCluster(t, rafts)
	}()

	startCluster()

	leader := waitForLeader(t, rafts, 10*time.Second)

	t.Logf("First leader elected: Node %d", leader.ID())

	t.Logf("Shutting down leader %d", leader.ID())
	leader.Shutdown()

	// Wait for a new leader to be elected
	newLeader := waitForLeader(t, rafts, 10*time.Second)

	t.Logf("New leader elected: Node %d", newLeader.ID())

	// Verify that exactly one leader was elected
	leaderCount := countLeaders(rafts)
	assert.Equal(t, 1, leaderCount, "Expected exactly one leader")
}

func TestLeaderElection_OnlyNodesWithLatestLog(t *testing.T) {
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

	fasterNodeThree := func(node *testNode) {
		if node.id != 3 {
			return
		}

		t.Logf("Ensuring node 3 attempts a leader election before other nodes.")
		node.options.MinElectionTimeout = 700 * time.Millisecond
		node.options.MaxElectionTimout = 1300 * time.Millisecond
	}

	rafts, startCluster := setupCluster(t, 3, populateLogs, fasterNodeThree)
	defer func() {
		cleanupTestCluster(t, rafts)
	}()

	startCluster()

	leader := waitForLeader(t, rafts, 15*time.Second)

	require.NotEqual(t, 3, leader.ID(), "Expected a leader other than node 3")
}

func TestLeaderElection_OnNetworkPartition(t *testing.T) {
	partitionCluster := func(node *testNode) {
		if node.id > 7 {
			lostNetwork := latency.Network{
				Kbps:    0,              // Setting bandwidth to 0 for complete blocking
				Latency: 24 * time.Hour, // Extremely high latency that effectively blocks communication
				MTU:     0,              // Setting MTU to 0 to prevent any packet transmission
			}

			list, err := net.Listen("tcp", node.addr)
			require.NoError(t, err, "Failed to listen on address %s", node.addr)
			node.transport = transport.NewGRPCTransport(lostNetwork.Listener(list), &transport.GRPCTransportConfig{
				Dialer: func(_ context.Context, target string) (net.Conn, error) {
					conn, err := net.Dial("tcp", target)
					if err != nil {
						return nil, err
					}

					return lostNetwork.Conn(conn)
				},
			})

			node.options.MinElectionTimeout = 500 * time.Millisecond
			node.options.MaxElectionTimout = 1 * time.Second
		}
	}

	nodes, startCluster := setupCluster(t, 10, partitionCluster)
	defer func() {
		cleanupTestCluster(t, nodes)
	}()

	startCluster()

	leader := waitForLeader(t, nodes, 10*time.Second)

	t.Logf("Leader elected: Node %d", leader.ID())

	require.LessOrEqual(t, leader.ID(), uint64(7), "Expected leader to be on node 7 or less")

	// Verify that exactly one leader was elected
	leaderCount := countLeaders(nodes)
	assert.Equal(t, 1, leaderCount, "Expected exactly one leader")
}

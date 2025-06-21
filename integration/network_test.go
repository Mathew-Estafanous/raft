package integration

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/benchmark/latency"
)

func TestNetwork_RequestLatency(t *testing.T) {
	tests := []struct {
		name    string
		latency latency.Network
	}{
		{"SmallDelay", latency.LAN},
		{"MediumDelay", latency.WAN},
		{"LargeDelay", latency.Longhaul},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			networkDelay := func(node *testNode) {
				node.options.ForwardApply = true
				node.list = tt.latency.Listener(node.list)
				node.options.Dialer = func(_ context.Context, target string) (net.Conn, error) {
					conn, err := net.Dial("tcp", target)
					if err != nil {
						return nil, err
					}

					return tt.latency.Conn(conn)
				}
			}

			nodes, startCluster := setupCluster(t, 7, networkDelay)
			defer func() {
				cleanupTestCluster(t, nodes)
			}()

			startCluster()

			leader, err := waitForLeader(t, nodes, 15*time.Second)
			require.NoError(t, err)
			require.NotNil(t, leader)

			for i := 0; i < 5; i++ {
				n := nodes[rand.Intn(len(nodes))]
				t.Logf("Sending command %d to node %d", i, n.id)
				task := n.raft.Apply([]byte("cmd" + strconv.Itoa(i)))
				require.NoError(t, task.Error())
			}
		})
	}
}

func TestNetwork_PartitionRecovery(t *testing.T) {
	partitionEnabled := true
	partition := func(node *testNode) {
		lostNetwork := latency.Network{
			Kbps:    0,
			Latency: 24 * time.Hour,
			MTU:     0,
		}

		if node.id > 5 {
			node.list = latency.Local.Listener(node.list)
			node.options.Dialer = func(_ context.Context, target string) (net.Conn, error) {
				conn, err := net.Dial("tcp", target)
				if err != nil {
					return nil, err
				}

				allowedTargets := []string{
					"127.0.0.1:9206",
					"127.0.0.1:9207",
				}
				if slices.Contains(allowedTargets, target) || !partitionEnabled {
					return conn, nil
				}

				return lostNetwork.Conn(conn)
			}
		} else {
			node.list = latency.Local.Listener(node.list)
			node.options.Dialer = func(_ context.Context, target string) (net.Conn, error) {
				conn, err := net.Dial("tcp", target)
				if err != nil {
					return nil, err
				}

				allowedTargets := []string{
					"127.0.0.1:9201",
					"127.0.0.1:9202",
					"127.0.0.1:9203",
					"127.0.0.1:9204",
					"127.0.0.1:9205",
				}
				if slices.Contains(allowedTargets, target) || !partitionEnabled {
					return conn, nil
				}

				return lostNetwork.Conn(conn)
			}
		}
	}

	nodes, startCluster := setupCluster(t, 7, partition)
	defer func() {
		cleanupTestCluster(t, nodes)
	}()

	startCluster()

	leader, err := waitForLeader(t, nodes, 10*time.Second)
	require.NoError(t, err)
	require.NotNil(t, leader)

	t.Logf("Elected leader: Node %d", leader.ID())

	task := leader.Apply([]byte("cmd1"))
	require.NoError(t, task.Error(), "Failed to apply command to leader")

	task = leader.Apply([]byte("cmd2"))
	require.NoError(t, task.Error(), "Failed to apply command to leader")

	t.Logf("Disabling network partition and waiting for recovery")

	partitionEnabled = false

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for {
		allNodesUpdated := true
		for _, node := range nodes {
			logs, err := node.logStore.AllLogs()
			if err != nil {
				require.NoError(t, err, "Failed to get logs from node %d", node.id)
				allNodesUpdated = false
				break
			}

			if len(logs) < 3 {
				allNodesUpdated = false
				break
			}

			for i := 1; i <= 2; i++ {
				if !bytes.Equal(logs[i].Cmd, []byte(fmt.Sprintf("cmd%d", i))) {
					allNodesUpdated = false
					break
				}
			}
		}

		if allNodesUpdated {
			break
		}

		select {
		case <-ctx.Done():
			t.Fatal("Timed out waiting for partition recovery")
		case <-time.After(100 * time.Millisecond):
			continue
		}
	}
}

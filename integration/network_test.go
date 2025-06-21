package integration

import (
	"context"
	"math/rand"
	"net"
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

package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	mathrand "math/rand"
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
				node.grpcConfig.Dialer = func(_ context.Context, target string) (net.Conn, error) {
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
				n := nodes[mathrand.Intn(len(nodes))]
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
			node.grpcConfig.Dialer = func(_ context.Context, target string) (net.Conn, error) {
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
			node.grpcConfig.Dialer = func(_ context.Context, target string) (net.Conn, error) {
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

// generateTLSConfig creates a TLS configuration for testing with a self-signed certificate
func generateTLSConfig(t *testing.T, id string, certPool *x509.CertPool) *tls.Config {
	t.Helper()

	// Generate a private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Create a certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   fmt.Sprintf("Raft-%v", id),
			Organization: []string{"Test Raft Cluster"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	// Create the certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	// Encode the certificate to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	// Create a certificate pool and add the certificate
	certPool.AppendCertsFromPEM(certPEM)

	// Create the TLS configuration
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{
			{
				Certificate: [][]byte{certDER},
				PrivateKey:  privateKey,
			},
		},
		RootCAs:   certPool,
		ClientCAs: certPool,
	}
	return tlsConfig
}

func TestNetwork_TLSEncryption(t *testing.T) {
	// Setup a cluster with TLS configuration
	certPool := x509.NewCertPool()
	tlsEnabled := func(node *testNode) {
		node.grpcConfig.TLSConfig = generateTLSConfig(t, strconv.FormatUint(node.id, 10), certPool)
	}

	nodes, startCluster := setupCluster(t, 3, tlsEnabled)
	defer func() {
		cleanupTestCluster(t, nodes)
	}()

	startCluster()

	// Wait for a leader to be elected
	leader, err := waitForLeader(t, nodes, 5*time.Second)
	require.NoError(t, err)
	require.NotNil(t, leader)

	t.Logf("Elected leader: Node %d", leader.ID())

	// Apply commands to the cluster and verify they are replicated
	for i := 0; i < 3; i++ {
		cmdData := []byte(fmt.Sprintf("tls-cmd-%d", i))
		task := leader.Apply(cmdData)
		require.NoError(t, task.Error(), "Failed to apply command to leader")
	}
}

func TestNetwork_mTLS_EnabledAuthentication(t *testing.T) {
	certPool1 := x509.NewCertPool()
	certPool2 := x509.NewCertPool()
	tlsEnabled := func(node *testNode) {
		node.options.ForwardApply = true
		if node.id <= 2 {
			node.grpcConfig.TLSConfig = generateTLSConfig(t, strconv.FormatUint(node.id, 10), certPool1)
		} else {
			node.grpcConfig.TLSConfig = generateTLSConfig(t, strconv.FormatUint(node.id, 10), certPool2)
		}
	}

	nodes, startCluster := setupCluster(t, 3, tlsEnabled)
	defer func() {
		cleanupTestCluster(t, nodes)
	}()

	startCluster()

	// Wait for a leader to be elected
	leader, err := waitForLeader(t, nodes, 5*time.Second)
	require.NoError(t, err)
	require.NotNil(t, leader)

	// confirm that node 3 is unable to participate in the cluster
	// because it is configured with a different TLS configuration
	n3 := nodes[2]
	task := n3.raft.Apply([]byte("cmd"))
	err = task.Error()
	t.Logf("Node 3 task response: %v", err)
	require.Error(t, err, "Node 3 should not be able to apply commands with a different TLS configuration")
}

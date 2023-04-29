package raft

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

// Cluster keeps track of all other nodes and their addresses.
// It also holds agreed upon constants such as heart beat time and election timeout.
type Cluster interface {
	AddNode(n Node) error
	GetNode(id uint64) (Node, error)
	AllNodes() map[uint64]Node
	Quorum() int
}

type Node struct {
	// An ID that uniquely identifies the raft in the Cluster.
	ID uint64 `json:"id"`

	// Address of the node, that other rafts can contact.
	Addr string `json:"addr"`
}

// StaticCluster is a static definition of the entire cluster. As such, new members
// in the cluster are not dynamically discovered and instead must be explicitly added.
type StaticCluster struct {
	mu sync.Mutex
	// AllLogs the nodes within the raft Cluster. Key is a raft id.
	Nodes  map[uint64]Node
	logger *log.Logger
}

func (c *StaticCluster) AllNodes() map[uint64]Node {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Nodes
}

// NewClusterWithConfig similarly creates a cluster and adds all the nodes that are
// defined by configuration reader. The config file formatting is expected to be a
// json format.
func NewClusterWithConfig(conf io.Reader) (*StaticCluster, error) {
	cl := &StaticCluster{
		Nodes:  make(map[uint64]Node),
		logger: log.New(os.Stdout, "[Cluster]", log.LstdFlags),
	}
	decoder := json.NewDecoder(conf)
	if err := decoder.Decode(&cl.Nodes); err != nil {
		return nil, err
	}
	return cl, nil
}

func (c *StaticCluster) AddNode(n Node) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.Nodes[n.ID]; ok {
		return fmt.Errorf("[staticCluster] A node with ID: %d is already registered", n.ID)
	}
	c.logger.Printf("Added a new node with ID: %d and Address: %v", n.ID, n.Addr)
	c.Nodes[n.ID] = n
	return nil
}

func (c *StaticCluster) GetNode(id uint64) (Node, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n, ok := c.Nodes[id]
	if !ok {
		return Node{}, fmt.Errorf("couldn't find a node with id %v", id)
	}
	return n, nil
}

func (c *StaticCluster) Quorum() int {
	return len(c.Nodes)/2 + 1
}

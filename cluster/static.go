package cluster

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

// StaticCluster is a static definition of all members of the cluster. As such new members
// cannot be dynamically discovered. All members must be known from the start.
type StaticCluster struct {
	mu sync.Mutex
	// AllLogs the nodes within the raft Cluster. Key is a raft id.
	Nodes  map[uint64]Node
	Logger *log.Logger
}

func (c *StaticCluster) AllNodes() map[uint64]Node {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Nodes
}

// NewCluster will create an entirely new static that doesn't contain any nodes.
func NewCluster() *StaticCluster {
	return &StaticCluster{
		Nodes:  make(map[uint64]Node),
		Logger: log.New(os.Stdout, "[Cluster]", log.LstdFlags),
	}
}

// NewClusterWithConfig similarly creates a static and adds all the nodes that are
// defined by configuration reader. The config file formatting is expected to be a
// json format.
func NewClusterWithConfig(conf io.Reader) (*StaticCluster, error) {
	cl := NewCluster()
	decoder := json.NewDecoder(conf)
	if err := decoder.Decode(&cl.Nodes); err != nil {
		return nil, err
	}
	return cl, nil
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

func (c *StaticCluster) addNode(n Node) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.Nodes[n.ID]; ok {
		return fmt.Errorf("[Cluster] A node with ID: %d is already registered", n.ID)
	}
	c.Logger.Printf("Added a new node with ID: %d and Address: %v", n.ID, n.Addr)
	c.Nodes[n.ID] = n
	return nil
}

func (c *StaticCluster) removeNode(id uint64) (Node, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n, ok := c.Nodes[id]
	if !ok {
		return Node{}, fmt.Errorf("[Cluster] A node with ID: %d is not registered", id)
	}
	c.Logger.Printf("Removed a node with ID: %d and Address: %v", n.ID, n.Addr)
	delete(c.Nodes, n.ID)
	return n, nil
}

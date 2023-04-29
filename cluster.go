package raft

import (
	"encoding/json"
	"fmt"
	"github.com/Mathew-Estafanous/memlist"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

// Cluster keeps track of all other nodes and their addresses.
// It also holds agreed upon constants such as heart beat time and election timeout.
type Cluster interface {
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

// StaticCluster is a static definition of all members of the cluster. As such new members
// cannot be dynamically discovered. All members must be known from the start.
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

// NewCluster will create an entirely new static that doesn't contain any nodes.
func NewCluster() *StaticCluster {
	return &StaticCluster{
		Nodes:  make(map[uint64]Node),
		logger: log.New(os.Stdout, "[Cluster]", log.LstdFlags),
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

type DynamicCluster struct {
	cl     *StaticCluster
	member *memlist.Member
	logger *log.Logger
}

func (c *DynamicCluster) OnMembershipChange(peer memlist.Node) {
	node := Node{}
	switch peer.State {
	case memlist.Alive:
		err := addNode(c.cl, node)
		if err != nil {
			return
		}
	}
}

func (c *DynamicCluster) Join(otherAddr string) error {
	return c.member.Join(otherAddr)
}

func (c *DynamicCluster) Leave() error {
	return c.member.Leave(time.Second * 10)
}

func addNode(cl *StaticCluster, n Node) error {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	if _, ok := cl.Nodes[n.ID]; ok {
		return fmt.Errorf("[Cluster] A node with ID: %d is already registered", n.ID)
	}
	cl.logger.Printf("Added a new node with ID: %d and Address: %v", n.ID, n.Addr)
	cl.Nodes[n.ID] = n
	return nil
}

func NewDynamicCluster(port uint16) (*DynamicCluster, error) {
	cluster := &DynamicCluster{
		cl:     NewCluster(),
		logger: log.New(os.Stdout, fmt.Sprintf("[Dynamic Cluster: %d]", port), log.LstdFlags),
	}
	config := memlist.DefaultLocalConfig()
	config.BindPort = port
	config.EventListener = cluster
	member, err := memlist.Create(config)
	if err != nil {
		return nil, err
	}
	cluster.member = member
	return cluster, nil
}

package raft

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"github.com/Mathew-Estafanous/memlist"
	"io"
	"log"
	"os"
	"strconv"
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

func (c *StaticCluster) addNode(n Node) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.Nodes[n.ID]; ok {
		return fmt.Errorf("[Cluster] A node with ID: %d is already registered", n.ID)
	}
	c.logger.Printf("[Cluster] Added a new node with ID: %d and Address: %v", n.ID, n.Addr)
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
	c.logger.Printf("[Cluster] Removed a node with ID: %d and Address: %v", n.ID, n.Addr)
	delete(c.Nodes, n.ID)
	return n, nil
}

type DynamicCluster struct {
	cl     *StaticCluster
	member *memlist.Member
	logger *log.Logger

	mu       sync.Mutex
	memIDMap map[string]uint64
}

func NewDynamicCluster(port uint16) (*DynamicCluster, error) {
	gob.Register(Node{})
	cluster := &DynamicCluster{
		cl:       NewCluster(),
		logger:   log.New(os.Stdout, fmt.Sprintf("[Dynamic Cluster :%d]", port), log.LstdFlags),
		memIDMap: make(map[string]uint64),
	}
	config := memlist.DefaultLocalConfig()
	config.Name = "Member #" + strconv.Itoa(int(port))
	config.BindPort = port
	config.EventListener = cluster
	member, err := memlist.Create(config)
	if err != nil {
		return nil, err
	}
	cluster.member = member
	return cluster, nil
}

func (c *DynamicCluster) GetNode(id uint64) (Node, error) {
	return c.cl.GetNode(id)
}

func (c *DynamicCluster) AllNodes() map[uint64]Node {
	return c.cl.AllNodes()
}

func (c *DynamicCluster) Quorum() int {
	return c.cl.Quorum()
}

func (c *DynamicCluster) OnMembershipChange(peer memlist.Node) {
	switch peer.State {
	case memlist.Alive:
		node, ok := peer.Data.(Node)
		if !ok {
			c.logger.Printf("Failed to get")
		}
		err := c.addNode(peer.Name, node)
		if err != nil {
			c.logger.Printf("Failed to add node: %v", err)
			return
		}
		c.logger.Printf("Added a new node with ID: %d and Address: %v", node.ID, node.Addr)
	case memlist.Left, memlist.Dead:
		node, err := c.removeNode(peer.Name)
		if err != nil {
			c.logger.Printf("Failed to remove node: %v", err)
			return
		}
		c.logger.Printf("Removed a node with ID: %d and Address: %v", node.ID, node.Addr)
	}
}

func (c *DynamicCluster) Join(otherAddr string, raftNode Node) error {
	err := c.cl.addNode(raftNode)
	if err != nil {
		return err
	}
	return c.member.Join(otherAddr, raftNode)
}

func (c *DynamicCluster) Leave() error {
	return c.member.Leave(time.Second * 10)
}

func (c *DynamicCluster) addNode(name string, node Node) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.memIDMap[name] = node.ID
	return c.cl.addNode(node)
}

func (c *DynamicCluster) removeNode(name string) (Node, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	id, found := c.memIDMap[name]
	if !found {
		return Node{}, fmt.Errorf("couldn't find member with name %v", name)
	}
	delete(c.memIDMap, name)
	return c.cl.removeNode(id)
}

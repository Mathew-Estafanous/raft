package cluster

import (
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/Mathew-Estafanous/memlist"
)

// DynamicCluster provides a dynamic membership solution for a Raft cluster.
// It internally uses the memlist library to handle node discovery, failure detection,
// and membership changes. DynamicCluster allows  nodes to join and leave the cluster at runtime,
// automatically propagating membership information to other nodes in the cluster.
type DynamicCluster struct {
	cl     *StaticCluster
	member *memlist.Member
	logger *log.Logger
}

func NewDynamicCluster(addr string, port uint16, raftNode Node) (*DynamicCluster, error) {
	gob.Register(Node{})
	cluster := &DynamicCluster{
		cl:     NewCluster(),
		logger: log.New(os.Stdout, fmt.Sprintf("[Dynamic Cluster :%d]", port), log.LstdFlags),
	}
	config := memlist.DefaultLocalConfig()
	config.Name = "M#" + strconv.Itoa(int(port))
	config.BindAddr = addr
	config.BindPort = port
	config.EventListener = cluster
	config.MetaData = raftNode

	member, err := memlist.Create(config)
	if err != nil {
		return nil, err
	}
	cluster.member = member

	err = cluster.cl.addNode(raftNode)
	if err != nil {
		return nil, err
	}
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
	node, ok := peer.Data.(Node)
	if !ok {
		c.logger.Printf("Failed to get member node Data: %v", peer.Data)
	}
	switch peer.State {
	case memlist.Alive:
		err := c.cl.addNode(node)
		if err != nil {
			c.logger.Printf("Failed to add node: %v", err)
			return
		}
	case memlist.Left, memlist.Dead:
		_, err := c.cl.removeNode(node.ID)
		if err != nil {
			c.logger.Printf("Failed to remove node: %v", err)
			return
		}
	}
}

// Join will initiate the joining process of the raft node to the cluster.
func (c *DynamicCluster) Join(otherAddr string) error {
	return c.member.Join(otherAddr)
}

func (c *DynamicCluster) Leave(timeout time.Duration) error {
	return c.member.Leave(timeout)
}

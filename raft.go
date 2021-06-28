package raft

import (
	"fmt"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
	"time"
)

type Role byte

const (
	Follower  = 'F'
	Candidate = 'C'
	Leader    = 'L'
)

type node struct {
	// An ID that uniquely identifies the raft in the cluster.
	ID uint64

	// Address of the node, that other rafts can contact.
	Addr string
}

type cluster struct {
	Nodes map[uint64]node
	logger *log.Logger
}

func NewCluster() *cluster {
	return &cluster{
		Nodes: make(map[uint64]node),
		logger: log.New(os.Stdout, "[Cluster]", log.LstdFlags),
	}
}

func (c *cluster) addNode(n node) error {
	if _, ok := c.Nodes[n.ID]; ok {
		return fmt.Errorf("[Cluster] A node with %d is already registered", n.ID)
	}
	c.logger.Printf("Added a new node with ID: %d and Address: %v", n.ID, n.Addr)
	c.Nodes[n.ID] = n
	return nil
}

type Raft struct {
	raftId uint64
	timer  time.Timer
	logger *log.Logger

	// Each raft is part of a cluster that keeps track of all other
	// nodes and their address location.
	cluster *cluster

	currentTerm uint64
	votedFor    uint64
	role        Role
}

// NewRaft creates a new raft node and registers it with the provided cluster.
func NewRaft(c *cluster, id uint64) (*Raft, error) {
	if id == 0 {
		return nil, fmt.Errorf("A raft ID cannot be 0, choose a different ID")
	}
	logger := log.New(os.Stdout, fmt.Sprintf("[Raft: %d]", id), log.LstdFlags)
	return &Raft{
		raftId:      id,
		logger:      logger,
		cluster:     c,
		currentTerm: 0,
		votedFor:    0,
		role:        Follower,
	}, nil
}

func (r Raft) Serve(l net.Listener) error {
	n := node{
		ID:   r.raftId,
		Addr: l.Addr().String(),
	}
	err := r.cluster.addNode(n)
	if err != nil {
		return err
	}

	server := grpc.NewServer()
	r.logger.Printf("Starting raft on %v", l.Addr().String())
	err = server.Serve(l)
	if err != nil {
		r.logger.Printf("Failed to to server.")
		return err
	}
	return nil
}
package raft

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"
)

type StateType byte

const (
	Follower  StateType = 'F'
	Candidate           = 'C'
	Leader              = 'L'
)

type State interface {
	runState()
	getType() StateType
}

type node struct {
	// An ID that uniquely identifies the raft in the cluster.
	ID uint64

	// Address of the node, that other rafts can contact.
	Addr string
}

type cluster struct {
	Nodes  map[uint64]node
	logger *log.Logger
}

func NewCluster() *cluster {
	return &cluster{
		Nodes:  make(map[uint64]node),
		logger: log.New(os.Stdout, "[Cluster]", log.LstdFlags),
	}
}

func (c *cluster) addNode(n node) error {
	if _, ok := c.Nodes[n.ID]; ok {
		return fmt.Errorf("[Cluster] A node with ID: %d is already registered", n.ID)
	}
	c.logger.Printf("Added a new node with ID: %d and Address: %v", n.ID, n.Addr)
	c.Nodes[n.ID] = n
	return nil
}

func (c *cluster) quorum() int {
	return len(c.Nodes)/2 + 1
}

type Raft struct {
	id     uint64
	timer  *time.Timer
	logger *log.Logger

	// Each raft is part of a cluster that keeps track of all other
	// nodes and their address location.
	cluster *cluster

	mu			sync.Mutex
	currentTerm uint64
	state       State

	shutdownCh chan bool
}

// NewRaft creates a new raft node and registers it with the provided cluster.
func NewRaft(c *cluster, id uint64) (*Raft, error) {
	if id == 0 {
		return nil, fmt.Errorf("A raft ID cannot be 0, choose a different ID")
	}
	logger := log.New(os.Stdout, fmt.Sprintf("[Raft: %d]", id), log.LstdFlags)
	r := &Raft{
		id:          id,
		timer:       time.NewTimer(1 * time.Second),
		logger:      logger,
		cluster:     c,
		currentTerm: 0,
	}
	r.state = &follower{Raft: r}
	return r, nil
}

func (r *Raft) ListenAndServe(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return r.Serve(lis)
}

func (r *Raft) Serve(l net.Listener) error {
	n := node{
		ID:   r.id,
		Addr: l.Addr().String(),
	}
	err := r.cluster.addNode(n)
	if err != nil {
		return err
	}

	s := newServer(r, l)
	r.logger.Printf("Starting raft on %v", l.Addr().String())
	go func() {
		err := s.serve()
		if err != nil {
			r.logger.Printf("gRPC server crashed unexpectedly: %v", err)
			r.shutdownCh <- true
		}
	}()

	go r.run()
	return nil
}

// run is where the core logic of the Raft lies. It is a long running routine that
// periodically checks for recent RPC messages or other events and handles them accordingly.
func (r *Raft) run() {
	for {
		select {
		case <-r.shutdownCh:
			// Raft has shutdown and should no-longer run
			return
		default:
		}

		r.state.runState()
	}
}

func (r *Raft) setState(s StateType) {
	r.logger.Printf("Changing state from %c -> %c", r.state.getType(), s)
	r.mu.Lock()
	defer r.mu.Unlock()
	switch s {
	case Follower:
		r.state = &follower{Raft: r}
	case Candidate:
	    r.state = &candidate{
			Raft:          r,
			electionTimer: time.NewTimer(1 * time.Second),
		}
	case Leader:
		// TODO: Make the leader struct.
		return
	default:
		log.Printf("Provided State type %c is not valid!", s)
		r.shutdownCh <- true
	}
}

type follower struct {
	*Raft
	votedFor uint64
}

func (f *follower) runState() {
	f.timer.Reset(randTime())
	for {
		select {
		case <-f.timer.C:
			f.logger.Println("Timeout event has occurred. Starting an election")
			f.setState(Candidate)
			return
		case <-f.shutdownCh:
			return
		}
	}
}

func (f *follower) getType() StateType {
	return Follower
}

type candidate struct {
	*Raft
	electionTimer *time.Timer
}

func (c *candidate) runState() {
	c.mu.Lock()
	c.electionTimer.Reset(randTime())
	c.currentTerm++
	c.logger.Printf("Candidate started election for term %v.", c.currentTerm)
	c.mu.Unlock()
	_ = 0
	_ = c.cluster.quorum()
	for {
		select {
		case <-c.electionTimer.C:
			c.logger.Println("Election has failed, restarting election.")
			return
		case <- c.shutdownCh:
			return
		}
	}
}

func (c *candidate) getType() StateType {
	return Candidate
}

func randTime() time.Duration {
	min := int64(150 * time.Millisecond)
	max := int64(300 * time.Millisecond)
	return time.Duration(rand.Int63n(max-min) + min)
}

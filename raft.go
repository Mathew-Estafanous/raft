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

type Role byte

const (
	Follower  Role = 'F'
	Candidate      = 'C'
	Leader         = 'L'
)

type RoleFunc interface {
	runRole()
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
		return fmt.Errorf("[Cluster] A node with %d is already registered", n.ID)
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

	currentTerm uint64
	rMu sync.Mutex
	role        Role

	shutdownCh chan bool
}

// NewRaft creates a new raft node and registers it with the provided cluster.
func NewRaft(c *cluster, id uint64) (*Raft, error) {
	if id == 0 {
		return nil, fmt.Errorf("A raft ID cannot be 0, choose a different ID")
	}
	logger := log.New(os.Stdout, fmt.Sprintf("[Raft: %d]", id), log.LstdFlags)
	return &Raft{
		id:          id,
		timer: 		 time.NewTimer(1 * time.Millisecond),
		logger:      logger,
		cluster:     c,
		currentTerm: 0,
		role:        Follower,
	}, nil
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

	server := newServer(r, l)
	r.logger.Printf("Starting raft on %v", l.Addr().String())
	go func() {
		err = server.start()
		if err != nil {
			r.logger.Printf("gRPC server shutdown unexpectedly.")
		}
		r.shutdownCh <- true
	}()
	go r.run()
	return nil
}

// run is where the core logic of the Raft lies. It is a long running routine that
// periodically checks for recent RPC messages or other events and handles them accordingly.
func (r *Raft) run() {
	roles := map[Role]RoleFunc{
		Follower: &follower{Raft: r},
		Candidate: &candidate{
			Raft: r,
			electionTimer: time.NewTimer(1 * time.Millisecond),
		},
	}
	for {
		select {
		case <-r.shutdownCh:
			// Raft has shutdown and should no-longer run
			return
		default:
		}

		ro, ok := roles[r.role]
		if !ok {
			r.logger.Fatal("Role for %v could not be identified in map!", r.role)
		}
		ro.runRole()
	}
}

func (r *Raft) setRole(ro Role) {
	r.rMu.Lock()
	r.logger.Printf("Changing role from %c -> %c", r.role, ro)
	r.role = ro
	r.rMu.Unlock()
}

type follower struct {
	*Raft
	votedFor uint64
}

func (r *follower) runRole() {
	r.timer.Reset(randTime())
	for {
		select {
		case <-r.timer.C:
			r.logger.Println("Timeout event has occurred. Starting an election")
			r.setRole(Candidate)
			return
		case <-r.shutdownCh:
			return
		}
	}
}

type candidate struct {
	*Raft
	electionTimer *time.Timer
}

func (r *candidate) runRole() {
	r.electionTimer.Reset(randTime())
	r.currentTerm++
	r.logger.Printf("Candidate started election for term %v.", r.currentTerm)
	_ = 0
	_ = r.cluster.quorum()
	for {
		select {
		case <- r.electionTimer.C:
			r.logger.Println("Election has failed, restarting election.")
			return
		}
	}
}

func randTime() time.Duration {
	min := int64(150 * time.Millisecond)
	max := int64(300 * time.Millisecond)
	return time.Duration(rand.Int63n(max-min) + min)
}

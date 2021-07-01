package raft

import (
	"context"
	"fmt"
	"github.com/Mathew-Estafanous/raft/pb"
	"google.golang.org/grpc"
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
	Candidate StateType = 'C'
	Leader    StateType = 'L'
)

type State interface {
	runState()
	getType() StateType
	handleRPC(req interface{}) RPCResponse
}

type node struct {
	// An ID that uniquely identifies the raft in the cluster.
	ID uint64

	// Address of the node, that other rafts can contact.
	Addr string
}

// Each raft is part of a cluster that keeps track of all other
// nodes and their addresses. It also holds agreed upon constants
// such as heart beat time and election timeout.
type cluster struct {
	// Range of possible timeouts for elections or for
	// no heartbeats from the leader.
	minTimeout    time.Duration
	maxTimeout    time.Duration

	// Set time between heart beats (append entries) that the leader
	// should send out.
	heartBeatTime time.Duration

	// All the nodes within the raft cluster. Key is an raft id.
	Nodes  map[uint64]node
	logger *log.Logger
}

func NewCluster() *cluster {
	return &cluster{
		minTimeout:    1 * time.Second,
		maxTimeout:    3 * time.Second,
		heartBeatTime: 500 * time.Millisecond,
		Nodes:  make(map[uint64]node),
		logger: log.New(os.Stdout, "[Cluster]", log.LstdFlags),
	}
}

func (c *cluster) randElectTime() time.Duration {
	max := int64(c.maxTimeout)
	min := int64(c.minTimeout)
	return time.Duration(rand.Int63n(max-min) + min)
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

	mu          sync.Mutex
	cluster *cluster

	leaderId uint64
	currentTerm uint64
	state       State

	votedFor uint64
	voteTerm int64

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
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state.getType() == s {
		return
	}

	r.logger.Printf("Changing state from %c -> %c", r.state.getType(), s)
	switch s {
	case Follower:
		r.state = &follower{Raft: r}
	case Candidate:
		r.state = &candidate{
			Raft:          r,
			electionTimer: time.NewTimer(1 * time.Second),
		}
	case Leader:
		r.state = &leader{
			Raft:      r,
			heartbeat: time.NewTimer(1 * time.Second),
		}
	default:
		log.Fatalf("[BUG] Provided State type %c is not valid!", s)
	}
}

func (r *Raft) getState() State {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

func (r *Raft) onRequestVote(req *pb.VoteRequest) *pb.VoteResponse {
	r.mu.Lock()
	resp := &pb.VoteResponse{
		Term:        r.currentTerm,
		VoteGranted: false,
	}
	r.mu.Unlock()

	r.logger.Printf("Received a request vote from candidate %d for term: %d", req.CandidateId, req.Term)
	if req.Term < r.currentTerm {
		r.logger.Printf("Vote denied. Candidate term %d | Current term is %d", req.Term, r.currentTerm)
		return resp
	}

	if req.Term > r.currentTerm {
		r.mu.Lock()
		r.currentTerm = req.Term
		r.mu.Unlock()
		r.setState(Follower)

		resp.Term = req.Term
		r.votedFor = 0
	}

	if r.votedFor == 0 && r.voteTerm != int64(req.Term) {
		r.logger.Printf("Vote granted to candidate %d for term %d", req.CandidateId, req.Term)
		r.mu.Lock()
		r.voteTerm = int64(req.Term)
		r.timer.Reset(r.cluster.randElectTime())
		r.mu.Unlock()
		resp.VoteGranted = true
	}
	return resp
}

func (r *Raft) onAppendEntry(req *pb.AppendEntriesRequest) *pb.AppendEntriesResponse {
	r.timer.Reset(r.cluster.randElectTime())
	r.mu.Lock()
	resp := &pb.AppendEntriesResponse{
		Term:    r.currentTerm,
		Success: false,
	}

	if req.Term < r.currentTerm {
		r.logger.Printf("Append entry rejected since leader term: %d < current: %d", req.Term, r.currentTerm)
		return resp
	} else if req.Term > r.currentTerm {
		r.currentTerm = req.Term
	}
	r.mu.Unlock()
	r.setState(Follower)
	r.mu.Lock()
	if r.leaderId != req.LeaderId {
		r.logger.Printf("New leader ID: %d for term %d", req.LeaderId, r.currentTerm)
		r.leaderId = req.LeaderId
	}
	r.mu.Unlock()
	resp.Success = true
	return resp
}

func (r *Raft) sendRPC(req interface{}, target node) RPCResponse {
	conn, err := grpc.Dial(target.Addr, grpc.WithInsecure())
	if err != nil {
		return ToRPCResponse(nil, err)
	}
	c := pb.NewRaftClient(conn)

	var res interface{}
	switch req := req.(type) {
	case *pb.VoteRequest:
		res, err = c.RequestVote(context.Background(), req)
	case *pb.AppendEntriesRequest:
		res, err = c.AppendEntry(context.Background(), req)
	default:
		log.Fatalf("[BUG] Could not determine RPC request of %v", req)
	}
	return ToRPCResponse(res, err)
}

func (r *Raft) handleRPC(req interface{}) RPCResponse {
	var rpcErr error
	var resp interface{}

	switch req := req.(type) {
	case *pb.VoteRequest:
		resp = r.onRequestVote(req)
	case *pb.AppendEntriesRequest:
		resp = r.onAppendEntry(req)
	default:
		rpcErr = fmt.Errorf("unable to response to rpc request")
	}

	return ToRPCResponse(resp, rpcErr)
}

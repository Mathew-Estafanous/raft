package raft

import (
	"context"
	"errors"
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

var (
	// ErrRaftShutdown is thrown when a client operation has been issued after
	// the raft instance has shutdown.
	ErrRaftShutdown = errors.New("raft has already shutdown")

	// ErrNotLeader is thrown when a follower/candidate is given some operation
	// that only a leader is permitted to execute.
	ErrNotLeader = errors.New("this node is not a leader")
)

type raftState byte

const (
	Follower  raftState = 'F'
	Candidate raftState = 'C'
	Leader    raftState = 'L'
)

type state interface {
	runState()
	getType() raftState
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
	minTimeout time.Duration
	maxTimeout time.Duration

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
		Nodes:         make(map[uint64]node),
		logger:        log.New(os.Stdout, "[Cluster]", log.LstdFlags),
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

	mu      sync.Mutex
	cluster *cluster
	fsm     FSM

	leaderId uint64
	state    state

	// Persistent state of the raft.
	logMu 		sync.Mutex
	log         []*Log
	currentTerm uint64
	votedFor    uint64
	voteTerm    int64

	// Volatile state of the raft.
	commitIndex int64
	lastApplied int64

	shutdownCh  chan bool
	fsmUpdateCh chan fsmUpdate
	applyCh     chan *logTask
}

// New creates a new raft node and registers it with the provided cluster.
func New(c *cluster, id uint64, fsm FSM) (*Raft, error) {
	if id == 0 {
		return nil, fmt.Errorf("A raft ID cannot be 0, choose a different ID")
	}
	logger := log.New(os.Stdout, fmt.Sprintf("[Raft: %d]", id), log.LstdFlags)
	r := &Raft{
		id:          id,
		timer:       time.NewTimer(1 * time.Second),
		logger:      logger,
		cluster:     c,
		fsm:         fsm,
		currentTerm: 0,
		shutdownCh:  make(chan bool),
		fsmUpdateCh: make(chan Task),
		applyCh:     make(chan *logTask),
	}
	r.state = &follower{Raft: r}
	return r, nil
}

// ListenAndServe will start the raft instance and listen using TCP. The listening
// on the address that is provided as an argument. Note that serving the raft instance
// is the same as Serve, so it is best to look into that method as well.
func (r *Raft) ListenAndServe(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return r.Serve(lis)
}

// Serve (as the name suggests) will start up the raft instance and listen using
// the provided the net.Listener.
//
// This is a blocking operation and will only return when the raft instance has Shutdown
// or a fatal error has occurred.
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
	defer s.shutdown()
	r.logger.Printf("Starting raft on %v", l.Addr().String())
	go func() {
		if err := s.serve(); err != nil {
			r.logger.Printf("gRPC server crashed unexpectedly: %v", err)
			r.Shutdown()
		}
	}()

	go r.runFSM()
	r.run()
	return nil
}

func (r *Raft) Shutdown() {
	r.logger.Println("Shutting down instance.")
	close(r.shutdownCh)
}

// Apply takes a command and attempts to propagate it to the FSM and
// all other replicas in the raft cluster. A Task is returned which can
// be used to wait on the completion of the task.
func (r *Raft) Apply(cmd []byte) Task {
	logT := &logTask{
		log: &Log{
			Cmd: cmd,
		},
		errCh: make(chan error),
	}

	select {
	case <-r.shutdownCh:
		logT.respond(ErrRaftShutdown)
	case r.applyCh <- logT:
	}
	return logT
}

func (r *Raft) run() {
	for {
		select {
		case <-r.shutdownCh:
			// Raft has shutdown and should no-longer run
			return
		default:
			r.state.runState()
		}
	}
}

func (r *Raft) setState(s raftState) {
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
		log.Fatalf("[BUG] Provided state type %c is not valid!", s)
	}
}

func (r *Raft) getState() state {
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
		r.mu.Unlock()
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

	// validate that the PrevLogIndex is not at the starting default index value.
	r.logMu.Lock()
	defer r.logMu.Unlock()
	lastIndex := int64(len(r.log) - 1)
	if req.PrevLogIndex != -1 {
		var prevTerm uint64
		if req.PrevLogIndex == lastIndex {
			prevTerm = r.log[lastIndex].Term
		} else {
			// If the last index is less than the previous entry index then it is guaranteed
			// that the terms will not match. We can return a unsuccessful response in that case.
			if lastIndex < req.PrevLogIndex {
				r.logger.Printf("Request prev. index %v is greater then last index %v", req.PrevLogIndex, lastIndex)
				return resp
			}
			prevTerm = r.log[req.PrevLogIndex].Term
		}

		if prevTerm != req.PrevLogTerm {
			r.logger.Printf("Request prev. term %v does not match log term %v", req.PrevLogTerm, prevTerm)
			return resp
		}
	}

	if len(req.Entries) > 0 {
		for _, entry := range req.Entries {
			nlog := &Log{
				Index: entry.Index,
				Term: entry.Term,
				Cmd: entry.Data,
			}
			// if the entry index is less or equal than last then go back
			// and remove all log entries from that index to the end.
			if nlog.Index <= lastIndex {
				r.log = r.log[:nlog.Index]
			}
			r.log = append(r.log, nlog)
		}
	}

	r.mu.Lock()
	// Check if the leader has committed any new entries. If so, then
	// peer can also commit those changes and push them to the state machine.
	if req.LeaderCommit > r.commitIndex {
		for i := r.commitIndex + 1; i <= req.LeaderCommit; i++ {
			fsmApply := fsmUpdate{cmd: r.log[i].Cmd}
			r.fsmUpdateCh <- fsmApply
		}
		r.commitIndex = req.LeaderCommit
	}
	r.mu.Unlock()

	resp.Success = true
	return resp
}

func (r *Raft) sendRPC(req interface{}, target node) rpcResp {
	conn, err := grpc.Dial(target.Addr, grpc.WithInsecure())
	if err != nil {
		return toRPCResponse(nil, err)
	}

	defer func() {
		if err = conn.Close(); err != nil {
			r.logger.Print("Encountered an issue when closing connection: %v", err)
		}
	}()
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
	return toRPCResponse(res, err)
}

func (r *Raft) handleRPC(req interface{}) rpcResp {
	var rpcErr error
	var resp interface{}

	switch req := req.(type) {
	case *pb.VoteRequest:
		resp = r.onRequestVote(req)
	case *pb.AppendEntriesRequest:
		resp = r.onAppendEntry(req)
	default:
		rpcErr = fmt.Errorf("unable to response to rpcResp request")
	}

	return toRPCResponse(resp, rpcErr)
}

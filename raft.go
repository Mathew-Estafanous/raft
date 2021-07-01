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

type config struct {
	minElectTime  time.Duration
	maxElectTime  time.Duration
	heartBeatTime time.Duration
}

func (c config) randElectTime() time.Duration {
	max := int64(c.maxElectTime)
	min := int64(c.minElectTime)
	return time.Duration(rand.Int63n(max-min) + min)
}

var defaultConfig = &config{
	minElectTime:  1 * time.Second,
	maxElectTime:  3 * time.Second,
	heartBeatTime: 500 * time.Millisecond,
}

type Raft struct {
	id     uint64
	timer  *time.Timer
	logger *log.Logger

	mu          sync.Mutex
	// Each raft is part of a cluster that keeps track of all other
	// nodes and their address location.
	cluster *cluster

	conf *config

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
		conf: defaultConfig,
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
		r.timer.Reset(r.conf.randElectTime())
		r.mu.Unlock()
		resp.VoteGranted = true
	}
	return resp
}

func (r *Raft) onAppendEntry(req *pb.AppendEntriesRequest) *pb.AppendEntriesResponse {
	r.timer.Reset(r.conf.randElectTime())
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

type follower struct {
	*Raft
}

func (f *follower) runState() {
	f.timer.Reset(f.conf.randElectTime())
	for f.getState().getType() == Follower {
		select {
		case <-f.timer.C:
			f.logger.Println("Timeout event has occurred.")
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

func (f *follower) handleRPC(req interface{}) RPCResponse {
	var rpcErr error
	var resp interface{}

	switch req := req.(type) {
	case *pb.VoteRequest:
		resp = f.onRequestVote(req)
	case *pb.AppendEntriesRequest:
		resp = f.onAppendEntry(req)
	default:
		rpcErr = fmt.Errorf("unable to response to rpc request")
	}

	return ToRPCResponse(resp, rpcErr)
}

type candidate struct {
	*Raft
	electionTimer *time.Timer
	votesNeeded   int
	voteCh        chan RPCResponse
}

func (c *candidate) getType() StateType {
	return Candidate
}

func (c *candidate) runState() {
	c.mu.Lock()
	c.electionTimer.Reset(c.conf.randElectTime())
	c.currentTerm++
	c.logger.Printf("Candidate started election for term %v.", c.currentTerm)
	c.mu.Unlock()

	// start the leader election by creating the vote request and sending an
	// RPC request to all the other nodes in separate goroutines.
	c.voteCh = make(chan RPCResponse, len(c.cluster.Nodes))
	c.votesNeeded = c.cluster.quorum() - 1
	c.voteTerm = int64(c.currentTerm)
	c.votedFor = c.id
	req := &pb.VoteRequest{
		Term:        c.currentTerm,
		CandidateId: c.id,
	}
	for _, v := range c.cluster.Nodes {
		if v.ID == c.id {
			continue
		}

		go func(n node) {
			res := c.sendRPC(req, n)
			c.voteCh <- res
		}(v)
	}

	for c.getState().getType() == Candidate {
		select {
		case <-c.electionTimer.C:
			c.logger.Printf("Election has failed for term %d", c.currentTerm)
			return
		case v := <-c.voteCh:
			if v.error != nil {
				c.logger.Printf("A vote request has failed: %v", v.error)
				break
			}
			vote := v.resp.(*pb.VoteResponse)

			// If term of peer is greater then go back to follower
			// and update current term to the peer's term.
			if vote.Term > c.currentTerm {
				c.logger.Println("Demoting since peer's term is greater than current term")
				c.mu.Lock()
				c.currentTerm = vote.Term
				c.mu.Unlock()
				c.setState(Follower)
				break
			}

			if vote.VoteGranted {
				c.votesNeeded--
				// Check if the total votes needed has been reached. If so
				// then election has passed and candidate is now the leader.
				if c.votesNeeded == 0 {
					c.setState(Leader)
				}
			}
		case <-c.shutdownCh:
			return
		}
	}
}

func (c *candidate) handleRPC(req interface{}) RPCResponse {
	var rpcErr error
	var resp interface{}

	switch req := req.(type) {
	case *pb.VoteRequest:
		resp = c.onRequestVote(req)
	case *pb.AppendEntriesRequest:
		resp = c.onAppendEntry(req)
	default:
		rpcErr = fmt.Errorf("unable to response to rpc request")
	}

	return ToRPCResponse(resp, rpcErr)
}

type leader struct {
	*Raft
	heartbeat *time.Timer
	appendEntryCh chan RPCResponse
}

func (l *leader) getType() StateType {
	return Leader
}

func (l *leader) runState() {
	l.heartbeat.Reset(l.conf.heartBeatTime)
	l.appendEntryCh = make(chan RPCResponse, len(l.cluster.Nodes))
	for l.getState().getType() == Leader {
		select {
		case <-l.heartbeat.C:
			l.mu.Lock()
			req := &pb.AppendEntriesRequest{
				Term: l.currentTerm,
				LeaderId: l.id,
			}
			l.mu.Unlock()

			for _, v := range l.cluster.Nodes {
				if v.ID != l.id {
					go func(n node) {
						r := l.sendRPC(req, n)
						l.appendEntryCh <- r
					}(v)
				}
			}
			l.heartbeat.Reset(l.conf.heartBeatTime)
		case ae := <- l.appendEntryCh:
			if ae.error != nil {
				l.logger.Printf("An append entry request has failed: %v", ae.error)
				break
			}
			aeResp := ae.resp.(*pb.AppendEntriesResponse)
			l.logger.Println(aeResp)
		case <- l.shutdownCh:
			return
		}
	}
}

func (l *leader) handleRPC(req interface{}) RPCResponse {
	var rpcErr error
	var resp interface{}

	switch req := req.(type) {
	case *pb.VoteRequest:
		resp = l.onRequestVote(req)
	case *pb.AppendEntriesRequest:
		resp = l.onAppendEntry(req)
	default:
		rpcErr = fmt.Errorf("unable to response to rpc request")
	}

	return ToRPCResponse(resp, rpcErr)
}

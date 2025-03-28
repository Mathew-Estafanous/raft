package raft

import (
	"errors"
	"fmt"
	"github.com/Mathew-Estafanous/raft/pb"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

var (
	// ErrRaftShutdown is thrown when a client operation has been issued after
	// the raft instance has shutdown.
	ErrRaftShutdown = errors.New("raft has already shutdown")

	// DefaultOpts provide a general baseline configuration setting for the raft
	// node such as election timeouts and log threshold.
	DefaultOpts = Options{
		MinElectionTimeout: 150 * time.Millisecond,
		MaxElectionTimout:  300 * time.Millisecond,
		HeartBeatTimout:    100 * time.Millisecond,
		SnapshotTimer:      1 * time.Second,
		LogThreshold:       200,
	}

	SlowOpts = Options{
		MinElectionTimeout: 1 * time.Second,
		MaxElectionTimout:  3 * time.Second,
		HeartBeatTimout:    500 * time.Millisecond,
		SnapshotTimer:      8 * time.Second,
		LogThreshold:       5,
	}

	keyCurrentTerm = []byte("currentTerm")
	keyVotedFor    = []byte("votedFor")
)

type raftState byte

const (
	Follower  raftState = 'F'
	Candidate raftState = 'C'
	Leader    raftState = 'L'
)

// LeaderError is an error that is returned when a request that is only meant for the leader is
// sent to a follower or candidate.
type LeaderError struct {
	LeaderId   uint64
	LeaderAddr string
}

func NewLeaderError(id uint64, addr string) *LeaderError {
	return &LeaderError{
		LeaderId:   id,
		LeaderAddr: addr,
	}
}

func (l *LeaderError) Error() string {
	return fmt.Sprintf("This node is not a leader. Leader's ID is %v", l.LeaderId)
}

// Options defines required constants that the raft will use while running.
//
// This library provides about some predefined options to use instead of defining
// your own options configurations.
type Options struct {
	// Range of possible timeouts for elections or for
	// no heartbeats from the leader.
	MinElectionTimeout time.Duration
	MaxElectionTimout  time.Duration

	// Set time between heart beats (append entries) that the leader
	// should send out.
	HeartBeatTimout time.Duration

	// SnapshotTimer is the period of time between the raft's attempts at making a
	// snapshot of the current state of the FSM. Although a snapshot is attempted periodically
	// it is not guaranteed that a snapshot will be completed unless the LogThreshold is met.
	SnapshotTimer time.Duration

	// LogThreshold represents the total number of log entries that should be reached
	// before log compaction (snapshot) is triggered.
	LogThreshold uint64
}

// Raft represents a node within the entire raft cluster. It contains the core logic
// of the consensus algorithm such as keeping track of leaders, replicated logs and
// other important state.
type Raft struct {
	id        uint64
	timer     *time.Timer
	snapTimer *time.Timer
	logger    *log.Logger

	mu      sync.Mutex
	cluster Cluster
	opts    Options
	fsm     FSM

	leaderId uint64
	state    raftState

	// Persistent state of the raft.
	log    LogStore
	stable StableStore

	// Volatile state of the raft.
	commitIndex int64
	lastApplied int64

	shutdownCh chan bool
	fsmCh      chan Task
	applyCh    chan *logTask

	// Candidate state variables
	electionTimer *time.Timer
	votesNeeded   int
	voteCh        chan rpcResp

	// Leader state variables
	heartbeat     *time.Timer
	appendEntryCh chan appendEntryResp
	indexMu       sync.Mutex
	nextIndex     map[uint64]int64
	matchIndex    map[uint64]int64
	tasks         map[int64]*logTask
}

// New creates a new raft node and registers it with the provided Cluster.
func New(c Cluster, id uint64, opts Options, fsm FSM, logStr LogStore, stableStr StableStore) (*Raft, error) {
	if id == 0 {
		return nil, fmt.Errorf("A raft ID cannot be 0, choose a different ID")
	}
	logger := log.New(os.Stdout, fmt.Sprintf("[Raft: %d]", id), log.LstdFlags)
	r := &Raft{
		id:          id,
		timer:       time.NewTimer(1 * time.Hour),
		snapTimer:   time.NewTimer(opts.SnapshotTimer),
		logger:      logger,
		cluster:     c,
		opts:        opts,
		fsm:         fsm,
		log:         logStr,
		stable:      stableStr,
		commitIndex: -1,
		lastApplied: -1,
		shutdownCh:  make(chan bool),
		fsmCh:       make(chan Task, 5),
		applyCh:     make(chan *logTask, 5),
	}
	r.state = Follower
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
	select {
	case <-r.shutdownCh:
	default:
		close(r.shutdownCh)
	}
}

// Apply takes a command and attempts to propagate it to the FSM and
// all other replicas in the raft Cluster. A Task is returned which can
// be used to wait on the completion of the task.
func (r *Raft) Apply(cmd []byte) Task {
	logT := &logTask{
		log: &Log{
			Type: Entry,
			Cmd:  cmd,
		},
		errorTask: errorTask{errCh: make(chan error)},
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
			switch r.state {
			case Follower:
				r.runFollowerState()
			case Candidate:
				r.runCandidateState()
			case Leader:
				r.runLeaderState()
			}
		}
	}
}

func (r *Raft) getState() raftState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

func (r *Raft) setState(s raftState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == s {
		return
	}

	r.logger.Printf("Changing state from %c -> %c", r.state, s)
	switch s {
	case Follower:
		r.state = Follower
	case Candidate:
		r.state = Candidate
		r.electionTimer = time.NewTimer(1 * time.Hour)
	case Leader:
		r.state = Leader
		r.heartbeat = time.NewTimer(r.opts.HeartBeatTimout)
		r.appendEntryCh = make(chan appendEntryResp, len(r.cluster.AllNodes()))
		r.nextIndex = make(map[uint64]int64)
		r.matchIndex = make(map[uint64]int64)
		r.tasks = make(map[int64]*logTask)
	default:
		log.Fatalf("[BUG] Provided state type %c is not valid!", s)
	}
}

func (r *Raft) randElectTime() time.Duration {
	maxV := int64(r.opts.MaxElectionTimout)
	minV := int64(r.opts.MinElectionTimeout)
	return time.Duration(rand.Int63n(maxV-minV) + minV)
}

// fromStableStore will fetch data related to the given key from the stable store.
// Most importantly, it assumes all data being retrieved is required and all non-nil
// errors are fatal.
func (r *Raft) fromStableStore(key []byte) uint64 {
	val, err := r.stable.Get(key)
	if err != nil {
		r.logger.Fatalln(err)
	}

	// if the returned byte slice is empty then we can assume a default of
	// setting the term to 0.
	if len(val) == 0 {
		val = []byte("0")
	}

	term, err := strconv.Atoi(string(val))
	if err != nil {
		r.logger.Fatalln(err)
	}
	return uint64(term)
}

func (r *Raft) setStableStore(key []byte, val uint64) {
	if err := r.stable.Set(key, []byte(strconv.Itoa(int(val)))); err != nil {
		r.logger.Fatalln(err)
	}
}

func (r *Raft) onRequestVote(req *pb.VoteRequest) *pb.VoteResponse {
	r.timer.Reset(r.randElectTime())
	resp := &pb.VoteResponse{
		Term:        r.fromStableStore(keyCurrentTerm),
		VoteGranted: false,
	}

	r.logger.Printf("Received a request vote from candidate %d for term: %d.", req.CandidateId, req.Term)
	if req.Term < r.fromStableStore(keyCurrentTerm) {
		r.logger.Printf("[Vote Denied] Candidate term %d | Current term is %d.", req.Term, r.fromStableStore(keyCurrentTerm))
		return resp
	}

	if req.Term > r.fromStableStore(keyCurrentTerm) {
		r.setStableStore(keyCurrentTerm, req.Term)
		r.setState(Follower)

		resp.Term = req.Term
		r.setStableStore(keyVotedFor, 0)
	}

	if r.fromStableStore(keyVotedFor) != 0 {
		r.logger.Printf("[Vote Denied] Already granted vote for term %v.", r.fromStableStore(keyCurrentTerm))
		return resp
	}

	lastIdx := r.log.LastIndex()
	lastTerm := r.log.LastTerm()
	if lastIdx > req.LastLogIndex || (lastTerm == req.LastLogTerm && lastIdx > req.LastLogIndex) {
		r.logger.Printf("[Vote Denied] Candidate's log term/index are not up to date.")
		return resp
	}

	r.logger.Printf("[Vote Granted] To candidate %d for term %d", req.CandidateId, req.Term)

	r.setStableStore(keyVotedFor, req.CandidateId)
	resp.VoteGranted = true
	return resp
}

func (r *Raft) onAppendEntry(req *pb.AppendEntriesRequest) *pb.AppendEntriesResponse {
	r.timer.Reset(r.randElectTime())
	resp := &pb.AppendEntriesResponse{
		Id:      r.id,
		Term:    r.fromStableStore(keyCurrentTerm),
		Success: false,
	}

	if req.Term < r.fromStableStore(keyCurrentTerm) {
		r.logger.Printf("Append entry rejected since leader term: %d < current: %d", req.Term, r.fromStableStore(keyCurrentTerm))
		return resp
	} else if req.Term > r.fromStableStore(keyCurrentTerm) {
		r.setStableStore(keyCurrentTerm, req.Term)
	}
	r.setState(Follower)

	r.mu.Lock()
	if r.leaderId != req.LeaderId {
		r.logger.Printf("New leader ID: %d for term %d", req.LeaderId, r.fromStableStore(keyCurrentTerm))
		r.leaderId = req.LeaderId
	}
	r.mu.Unlock()

	// validate that the PrevLogIndex is not at the starting default index value.
	lastIdx := r.log.LastIndex()
	if req.PrevLogIndex != -1 && lastIdx != -1 {
		var prevTerm uint64
		if req.PrevLogIndex == lastIdx {
			prevTerm = r.log.LastTerm()
		} else {
			// If the last index is less than the leader's previous log index then it's guaranteed
			// that the terms will not match. We can return a unsuccessful response in that case.
			if lastIdx < req.PrevLogIndex {
				r.logger.Printf("Request prev. index %v is greater then last index %v", req.PrevLogIndex, lastIdx)
				return resp
			}

			prevLog, err := r.log.GetLog(req.PrevLogIndex)
			if err != nil {
				r.logger.Printf("Unable to get log at previous index %v", req.PrevLogIndex)
				return resp
			}
			prevTerm = prevLog.Term
		}

		if prevTerm != req.PrevLogTerm {
			r.logger.Printf("Request prev. term %v does not match log term %v", req.PrevLogTerm, prevTerm)
			return resp
		}
	}

	if len(req.Entries) > 0 {
		newEntries := make([]*Log, 0)
		allEntries := entriesToLogs(req.Entries)
		for i, e := range allEntries {
			if e.Index > lastIdx {
				newEntries = allEntries[i:]
				break
			}

			logEntry, err := r.log.GetLog(e.Index)
			if err != nil {
				r.logger.Printf("Failed to get log at index %v", e.Index)
				return resp
			}

			// if the log entry term at the given index doesn't match with the entry's term
			// we must remove all logs at the index and beyond and replace it with the new ones.
			if e.Term != logEntry.Term {
				err = r.log.DeleteRange(logEntry.Index, lastIdx)
				if err != nil {
					r.logger.Printf("Failed to delete range %d - %d", logEntry.Index+1, lastIdx)
					return resp
				}
				newEntries = allEntries[i:]
				break
			}
		}

		// if newEntries is greater than 0 then there are new entries that we must add to the log.
		if n := len(newEntries); n > 0 {
			if err := r.log.AppendLogs(newEntries); err != nil {
				r.logger.Println("Failed to append new log entries to log store.")
				return resp
			}

			if logs, err := r.log.AllLogs(); err == nil {
				r.logger.Printf("Updated Log: %v", logs)
			}
		}
	}

	r.mu.Lock()
	// Check if the leader has committed any new entries. If so, then
	// peer can also commit those changes and push them to the state machine.
	if req.LeaderCommit > r.commitIndex {
		r.commitIndex = min(r.log.LastIndex(), req.LeaderCommit)
		r.applyLogs()
	}
	r.mu.Unlock()

	resp.Success = true
	return resp
}

func (r *Raft) onForwardApplyRequest(req *pb.ApplyRequest) *pb.ApplyResponse {
	task := r.Apply(req.Command)
	if err := task.Error(); err != nil {
		return &pb.ApplyResponse{
			Result: pb.ApplyResult_Failed,
		}
	}

	return &pb.ApplyResponse{
		Result: pb.ApplyResult_Committed,
	}
}

// applyLogs will apply the newly committed logs to the FSM. The logs that
// will be applied will be from the lastApplied to the recent commit index.
func (r *Raft) applyLogs() {
	for i := r.lastApplied + 1; i <= r.commitIndex; i++ {
		l, err := r.log.GetLog(i)
		if err != nil {
			r.logger.Printf("Failed to get log index %v to fsm.", i)
			return
		}

		switch l.Type {
		case Snapshot:
			restore := &fsmRestore{
				cmd:       l.Cmd,
				errorTask: errorTask{errCh: make(chan error)},
			}
			i = l.Index
			r.fsmCh <- restore
			if restore.Error() != nil {
				r.logger.Fatalln("Could not successfully restore log snapshot to FSM")
			}
		case Entry:
			update := &fsmUpdate{
				cmd:       l.Cmd,
				errorTask: errorTask{errCh: make(chan error)},
			}
			r.fsmCh <- update
			if update.Error() != nil {
				r.logger.Fatalln("Could not successfully apply log entry to FSM")
			}
		default:
			r.logger.Fatalf("Type %v is not a valid log type", l.Type)
		}
	}
	r.lastApplied = r.commitIndex
}

// onSnapshot is called periodically and will check to see if a snapshot should be
// created based off of the log-threshold. If the threshold is met then a snapshot
// will be made of the current state of the FSM.
func (r *Raft) onSnapshot() {
	r.snapTimer.Reset(r.opts.SnapshotTimer)
	logs, err := r.log.AllLogs()
	if err != nil {
		r.logger.Println("Failed to get all logs from persistence.")
		return
	}

	// don't make a snapshot if the length of the logs is below the set log threshold.
	if len(logs) < int(r.opts.LogThreshold) {
		return
	}

	snapTask := &fsmSnapshot{
		errorTask: errorTask{errCh: make(chan error)},
	}
	r.fsmCh <- snapTask
	if snapTask.Error() != nil {
		r.logger.Println("Failed to create a snapshot of the FSM.")
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if err = r.log.DeleteRange(logs[0].Index, logs[len(logs)-1].Index); err != nil {
		r.logger.Fatalln(err)
	}

	// Use the FSM state in a snapshot log that is added to the Log persistence storage.
	snapLog := &Log{
		Type:  Snapshot,
		Index: r.lastApplied,
		Term:  r.fromStableStore(keyCurrentTerm),
		Cmd:   snapTask.state,
	}
	if err = r.log.AppendLogs([]*Log{snapLog}); err != nil {
		r.logger.Fatalln(err)
	}

	idx := r.lastApplied - logs[0].Index
	if err = r.log.AppendLogs(logs[idx+1:]); err != nil {
		r.logger.Fatalln(err)
	}

	if logs, err := r.log.AllLogs(); err == nil {
		r.logger.Printf("Snapshot Logs: %v", logs)
	}
}

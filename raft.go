package raft

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/Mathew-Estafanous/raft/cluster"
)

var (
	// ErrRaftShutdown is thrown when a client operation has been issued after
	// the raft instance has shutdown.
	ErrRaftShutdown = errors.New("raft has already shutdown")

	DefaultOpts = Options{
		MinElectionTimeout: 1 * time.Second,
		MaxElectionTimout:  3 * time.Second,
		HeartBeatTimout:    500 * time.Millisecond,
		SnapshotTimer:      8 * time.Second,
		LogThreshold:       100,
		ForwardApply:       true,
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
	// before log compaction (snapshot) is triggered. A threshold of 0 means no snapshot creation.
	LogThreshold uint64

	// ForwardApply enables follower nodes to automatically forward apply requests to the leader
	// and response to the request without the caller needing to know who is a leader. Otherwise,
	// the request will fail with a LeaderError.
	ForwardApply bool
}

// Raft represents a node within the entire raft cluster. It contains the core logic
// of the consensus algorithm such as keeping track of leaders, replicated logs and
// other important state.
type Raft struct {
	id        uint64
	timer     *time.Timer
	snapTimer *time.Timer
	logger    *slog.Logger

	mu      sync.Mutex
	cluster cluster.Cluster
	opts    Options
	fsm     FSM

	leaderMu sync.Mutex
	leaderId uint64
	state    raftState
	stateCh  chan raftState

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
	votesNeeded int
	voteCh      chan rpcResponse[*VoteResponse]

	// Leader state variables
	heartbeat     *time.Timer
	appendEntryCh chan appendEntryResp
	indexMu       sync.Mutex
	nextIndex     map[uint64]int64
	matchIndex    map[uint64]int64
	tasks         map[int64]*logTask

	// Transport layer
	transport Transport
}

// New creates a new raft node and registers it with the provided Cluster.
func New(c cluster.Cluster, id uint64, opts Options, fsm FSM, logStr LogStore, stableStr StableStore, transport Transport) (*Raft, error) {
	if id == 0 {
		return nil, fmt.Errorf("A raft ID cannot be 0, choose a different ID")
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
	}))
	logger = logger.With("raft_id", id)
	r := &Raft{
		id:          id,
		timer:       time.NewTimer(1 * time.Hour),
		snapTimer:   time.NewTimer(opts.SnapshotTimer),
		logger:      logger,
		cluster:     c,
		opts:        opts,
		fsm:         fsm,
		state:       Follower,
		stateCh:     make(chan raftState, 1),
		log:         logStr,
		stable:      stableStr,
		commitIndex: -1,
		lastApplied: -1,
		shutdownCh:  make(chan bool),
		fsmCh:       make(chan Task, 5),
		applyCh:     make(chan *logTask, 10),
		transport:   transport,
	}

	// Register the Raft instance as the request handler
	if err := transport.RegisterRequestHandler(&rpcHandler{raft: r}); err != nil {
		return nil, fmt.Errorf("failed to register request handler: %v", err)
	}
	return r, nil
}

// Serve (as the name suggests) will start up the raft instance and listen using
// the transport layer.
//
// This is a blocking operation and will only return when the raft instance has Shutdown
// or a fatal error has occurred.
func (r *Raft) Serve() error {
	if r.transport == nil {
		return fmt.Errorf("no transport configured, use ListenAndServe or provide a transport during initialization")
	}

	r.logger.Info("Starting raft instance")

	// Start the transport in a separate goroutine
	go func() {
		if err := r.transport.Start(); err != nil {
			r.logger.Error("Transport layer crashed unexpectedly",
				slog.String("error", err.Error()),
			)
			r.Shutdown()
		}
	}()

	go r.runFSM()
	r.run()

	// Stop the transport when we're done
	if err := r.transport.Stop(); err != nil {
		r.logger.Error("Error stopping transport: %v",
			slog.String("error", err.Error()),
		)
		return err
	}

	return nil
}

func (r *Raft) Shutdown() {
	r.logger.Info("Shutting down instance.")
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
			Type: LogEntry,
			Cmd:  cmd,
		},
		errorTask: errorTask{errCh: make(chan error, 1)},
	}

	select {
	case <-r.shutdownCh:
		logT.respond(ErrRaftShutdown)
		return logT
	default:
		// raft is not shutdown, continue.
	}

	select {
	case r.applyCh <- logT:
	}
	return logT
}

// ID of the raft node.
func (r *Raft) ID() uint64 {
	return r.id
}

func (r *Raft) run() {
	for {
		select {
		case <-r.shutdownCh:
			// Raft has shutdown and should no-longer run
			return
		default:
			switch r.getState() {
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
	defer func() {
		r.mu.Unlock()
		r.stateCh <- s
	}()
	if r.state == s {
		return
	}

	r.logger.Debug("State changed",
		slog.String("old_state", string(r.state)),
		slog.String("new_state", string(s)),
	)

	switch s {
	case Follower:
		r.state = Follower
	case Candidate:
		r.state = Candidate
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

// fromStableStore will fetch data related to the given key from the stable store.
// Most importantly, it assumes all data being retrieved is required and all non-nil
// errors are fatal.
func (r *Raft) fromStableStore(key []byte) uint64 {
	val, err := r.stable.Get(key)
	if err != nil {
		r.logger.Error("Failed to get data from stable store",
			slog.String("key", string(key)),
			slog.String("error", err.Error()),
		)
		panic("stable store in unrecoverable state")
	}

	// if the returned byte slice is empty then we can assume a default of
	// setting the term to 0.
	if len(val) == 0 {
		val = []byte("0")
	}

	term, err := strconv.Atoi(string(val))
	if err != nil {
		r.logger.Error("Failed to get data from stable store",
			slog.String("key", string(key)),
			slog.String("term", string(val)),
			slog.String("error", err.Error()),
		)
		panic(fmt.Sprintf("malformed term value in stable store: %v", string(val)))
	}
	return uint64(term)
}

func (r *Raft) setStableStore(key []byte, val uint64) {
	if err := r.stable.Set(key, []byte(strconv.Itoa(int(val)))); err != nil {
		r.logger.Error("Failed to set data in stable store",
			slog.String("key", string(key)),
			slog.Uint64("value", val),
			slog.String("error", err.Error()),
		)
		panic("stable store in unrecoverable state")
	}
}

func (r *Raft) onRequestVote(req *VoteRequest) *VoteResponse {
	r.timer.Reset(randElectTime(r.opts.MinElectionTimeout, r.opts.MaxElectionTimout))
	currentTerm := r.fromStableStore(keyCurrentTerm)
	resp := &VoteResponse{
		Term:        currentTerm,
		VoteGranted: false,
	}

	r.logger.Info("Received request vote",
		slog.Uint64("candidate_id", req.CandidateId),
		slog.Uint64("term", req.Term),
	)

	if req.Term < currentTerm {
		r.logger.Debug("Vote denied - candidate term too low",
			slog.Uint64("candidate_id", req.CandidateId),
			slog.Uint64("candidate_term", req.Term),
			slog.Uint64("current_term", currentTerm),
		)
		return resp
	}

	if req.Term > currentTerm {
		r.setStableStore(keyCurrentTerm, req.Term)
		r.setState(Follower)

		resp.Term = req.Term
		r.setStableStore(keyVotedFor, 0)
	}

	if r.fromStableStore(keyVotedFor) != 0 {
		r.logger.Debug("Vote denied - already voted",
			slog.Uint64("current_term", currentTerm),
		)
		return resp
	}

	lastIdx := r.log.LastIndex()
	lastTerm := r.log.LastTerm()
	if lastIdx > req.LastLogIndex || (lastTerm == req.LastLogTerm && lastIdx > req.LastLogIndex) {
		r.logger.Debug("Vote denied - candidate log not up to date")
		return resp
	}

	r.logger.Info("Vote granted",
		slog.Uint64("candidate_id", req.CandidateId),
		slog.Uint64("term", req.Term))

	r.setStableStore(keyVotedFor, req.CandidateId)
	resp.VoteGranted = true
	return resp
}

func (r *Raft) onAppendEntry(req *AppendEntriesRequest) *AppendEntriesResponse {
	r.timer.Reset(randElectTime(r.opts.MinElectionTimeout, r.opts.MaxElectionTimout))
	resp := &AppendEntriesResponse{
		Id:      r.id,
		Term:    r.fromStableStore(keyCurrentTerm),
		Success: false,
	}

	if req.Term < r.fromStableStore(keyCurrentTerm) {
		r.logger.Debug("Append entry rejected - leader term too low",
			slog.Uint64("leader_term", req.Term),
			slog.Uint64("current_term", r.fromStableStore(keyCurrentTerm)))
		return resp
	} else if req.Term > r.fromStableStore(keyCurrentTerm) {
		r.setStableStore(keyCurrentTerm, req.Term)
	}

	r.setState(Follower)

	r.leaderMu.Lock()
	if r.leaderId != req.LeaderId {
		r.logger.Info("New leader",
			slog.Uint64("leader_id", req.LeaderId),
			slog.Uint64("term", r.fromStableStore(keyCurrentTerm)))
		r.leaderId = req.LeaderId
	}
	r.leaderMu.Unlock()

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
				r.logger.Debug("Request prev. index is greater than last index",
					slog.Int64("prev_index", req.PrevLogIndex),
					slog.Int64("last_index", lastIdx))
				return resp
			}

			prevLog, err := r.log.GetLog(req.PrevLogIndex)
			if err != nil {
				r.logger.Warn("Failed to get log at previous index",
					slog.Int64("prev_index", req.PrevLogIndex),
					slog.String("error", err.Error()),
				)
				return resp
			}
			prevTerm = prevLog.Term
		}

		if prevTerm != req.PrevLogTerm {
			r.logger.Debug("Log term mismatch",
				slog.Uint64("request_term", req.PrevLogTerm),
				slog.Uint64("log_term", prevTerm))
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
				r.logger.Warn("Failed to get log entry",
					slog.Int64("index", e.Index),
					slog.String("error", err.Error()),
				)
				return resp
			}

			// if the log entry term at the given index doesn't match with the entry's term
			// we must remove all logs at the index and beyond and replace it with the new ones.
			if e.Term != logEntry.Term {
				err = r.log.DeleteRange(logEntry.Index, lastIdx)
				if err != nil {
					r.logger.Warn(fmt.Sprintf("Failed to delete logs at index %v to %v", logEntry.Index, lastIdx),
						slog.String("error", err.Error()),
					)
					return resp
				}
				newEntries = allEntries[i:]
				break
			}
		}

		// if newEntries is greater than 0 then there are new entries that we must add to the log.
		if n := len(newEntries); n > 0 {
			if err := r.log.AppendLogs(newEntries); err != nil {
				r.logger.Warn("Failed to append new log entries",
					slog.String("error", err.Error()),
				)
				return resp
			}

			if logs, err := r.log.AllLogs(); err == nil {
				r.logger.Debug("Updated log entries",
					slog.Any("logs", logs))
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
func (r *Raft) onForwardApplyRequest(req *ApplyRequest) *ApplyResponse {
	task := r.Apply(req.Command)
	if err := task.Error(); err != nil {
		return &ApplyResponse{
			Result: Failed,
		}
	}

	return &ApplyResponse{
		Result: Committed,
	}
}

// applyLogs will apply the newly committed logs to the FSM. The logs that
// will be applied will be from the lastApplied to the recent commit index.
func (r *Raft) applyLogs() {
	for i := r.lastApplied + 1; i <= r.commitIndex; i++ {
		l, err := r.log.GetLog(i)
		if err != nil {
			r.logger.Warn(fmt.Sprintf("Failed to get log index %v to fsm.", i),
				slog.String("error", err.Error()),
			)
			return
		}

		switch l.Type {
		case LogSnapshot:
			restore := &fsmRestore{
				cmd:       l.Cmd,
				errorTask: errorTask{errCh: make(chan error)},
			}
			i = l.Index
			r.fsmCh <- restore
			if restore.Error() != nil {
				r.logger.Error("Could not restore log snapshot to FSM",
					slog.String("error", restore.Error().Error()),
				)
				panic("Could not restore log snapshot to FSM")
			}
		case LogEntry:
			update := &fsmUpdate{
				cmd:       l.Cmd,
				errorTask: errorTask{errCh: make(chan error)},
			}
			r.fsmCh <- update
			if update.Error() != nil {
				r.logger.Error("Could not apply log entry to FSM",
					slog.String("error", update.Error().Error()),
				)
				panic("Could not apply log entry to FSM")
			}
		default:
			r.logger.Error(fmt.Sprintf("Type %v is not a valid log type", l.Type))
			panic("invalid apply log type")
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
		r.logger.Warn("Failed to get logs from persistence layer.",
			slog.String("error", err.Error()),
		)
		return
	}

	// Don't make a snapshot if the length of the logs is below the set threshold or equal to 0.
	if len(logs) < int(r.opts.LogThreshold) || r.opts.LogThreshold == 0 {
		return
	}

	snapTask := &fsmSnapshot{
		errorTask: errorTask{errCh: make(chan error)},
	}
	r.fsmCh <- snapTask
	if snapTask.Error() != nil {
		r.logger.Warn("Failed to create a snapshot of the FSM.",
			slog.String("error", snapTask.Error().Error()),
		)
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if err = r.log.DeleteRange(logs[0].Index, logs[len(logs)-1].Index); err != nil {
		r.logger.Warn(fmt.Sprintf("Deleting logs from range %v to %v failed.", logs[0].Index, logs[len(logs)-1].Index),
			slog.String("error", err.Error()),
		)
		return
	}

	// Use the FSM state in a snapshot log that is added to the Log persistence storage.
	snapLog := &Log{
		Type:  LogSnapshot,
		Index: r.lastApplied,
		Term:  r.fromStableStore(keyCurrentTerm),
		Cmd:   snapTask.state,
	}
	if err = r.log.AppendLogs([]*Log{snapLog}); err != nil {
		r.logger.Error("Couldn't append new logs",
			slog.String("error", err.Error()),
		)
		panic("Couldn't append new logs")
	}

	idx := r.lastApplied - logs[0].Index
	if err = r.log.AppendLogs(logs[idx+1:]); err != nil {
		r.logger.Error("Couldn't append new logs",
			slog.String("error", err.Error()),
		)
		panic("Couldn't append new logs")
	}
}

func randElectTime(minTimeout, maxTimeout time.Duration) time.Duration {
	maxV := int64(maxTimeout)
	minV := int64(minTimeout)
	return time.Duration(rand.Int63n(maxV-minV) + minV)
}

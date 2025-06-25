package raft

import (
	"context"

	"github.com/Mathew-Estafanous/raft/cluster"
)

type Entry struct {
	Term  uint64
	Index int64
	Data  []byte
	Type  []byte
}

type AppendEntriesRequest struct {
	Term         uint64
	LeaderId     uint64
	PrevLogIndex int64
	PrevLogTerm  uint64
	Entries      []*Entry
	LeaderCommit int64
}

type AppendEntriesResponse struct {
	Id      uint64
	Term    uint64
	Success bool
}

type VoteRequest struct {
	Term         uint64
	CandidateId  uint64
	LastLogIndex int64
	LastLogTerm  uint64
}

type VoteResponse struct {
	Term        uint64
	VoteGranted bool
}

type ApplyResult int32

const (
	Committed ApplyResult = iota
	Failed
)

type ApplyRequest struct {
	Command []byte
}

type ApplyResponse struct {
	Result ApplyResult
}

// Transport defines the interface for communication between Raft nodes
type Transport interface {
	// Start initializes and starts the transport layer
	Start() error

	// Stop gracefully shuts down the transport layer
	Stop() error

	// SendVoteRequest sends a vote request to a target node
	SendVoteRequest(ctx context.Context, target cluster.Node, req *VoteRequest) (*VoteResponse, error)

	// SendAppendEntries sends append entries request to a target node
	SendAppendEntries(ctx context.Context, target cluster.Node, req *AppendEntriesRequest) (*AppendEntriesResponse, error)

	// SendApplyRequest forwards an apply request to a target node
	SendApplyRequest(ctx context.Context, target cluster.Node, req *ApplyRequest) (*ApplyResponse, error)

	// RegisterRequestHandler registers handlers for incoming requests
	RegisterRequestHandler(handler RequestHandler) error
}

// RequestHandler processes incoming requests
type RequestHandler interface {
	OnAppendEntry(req *AppendEntriesRequest) *AppendEntriesResponse
	OnRequestVote(req *VoteRequest) *VoteResponse
	OnForwardApplyRequest(req *ApplyRequest) *ApplyResponse
}

// rpcHandler is used as an adapter to implement RequestHandler for a Raft
// while ensuring methods remain private.
type rpcHandler struct {
	raft *Raft
}

func (r *rpcHandler) OnAppendEntry(req *AppendEntriesRequest) *AppendEntriesResponse {
	return r.raft.onAppendEntry(req)
}

func (r *rpcHandler) OnRequestVote(req *VoteRequest) *VoteResponse {
	return r.raft.onRequestVote(req)
}

func (r *rpcHandler) OnForwardApplyRequest(req *ApplyRequest) *ApplyResponse {
	return r.raft.onForwardApplyRequest(req)
}

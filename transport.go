package raft

import (
	"context"

	"github.com/Mathew-Estafanous/raft/cluster"
	"github.com/Mathew-Estafanous/raft/pb"
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
	onAppendEntry(req *pb.AppendEntriesRequest) *pb.AppendEntriesResponse
	onRequestVote(req *pb.VoteRequest) *pb.VoteResponse
	onForwardApplyRequest(req *pb.ApplyRequest) *pb.ApplyResponse
}

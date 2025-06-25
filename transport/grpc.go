package transport

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"time"

	"github.com/Mathew-Estafanous/raft"
	"github.com/Mathew-Estafanous/raft/cluster"
	"github.com/Mathew-Estafanous/raft/transport/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

type Dialer func(context.Context, string) (net.Conn, error)

// GRPCTransport implements the Transport interface using gRPC
type GRPCTransport struct {
	listener   net.Listener
	server     *grpc.Server
	tlsConfig  *tls.Config
	dialer     Dialer
	maxRetries int
	retryDelay time.Duration
	handler    raft.RequestHandler
}

// GRPCTransportConfig holds configuration for the gRPC transport
type GRPCTransportConfig struct {
	TLSConfig  *tls.Config
	Dialer     Dialer
	MaxRetries int
	RetryDelay time.Duration
}

// NewGRPCTransport creates a new gRPC transport
func NewGRPCTransport(listener net.Listener, config *GRPCTransportConfig) *GRPCTransport {
	if config == nil {
		config = &GRPCTransportConfig{}
	}

	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 40 * time.Millisecond
	}

	return &GRPCTransport{
		listener:   listener,
		tlsConfig:  config.TLSConfig,
		dialer:     config.Dialer,
		maxRetries: config.MaxRetries,
		retryDelay: config.RetryDelay,
	}
}

// Start initializes and starts the transport layer
func (t *GRPCTransport) Start() error {
	if t.handler == nil {
		return ErrNoHandlerRegistered
	}

	var rpcServer *grpc.Server
	if t.tlsConfig != nil {
		creds := credentials.NewTLS(t.tlsConfig)
		rpcServer = grpc.NewServer(grpc.Creds(creds))
	} else {
		rpcServer = grpc.NewServer()
	}

	t.server = rpcServer
	pb.RegisterRaftServer(t.server, &grpcTransportServer{r: t.handler})

	return t.server.Serve(t.listener)
}

// Stop gracefully shuts down the transport layer
func (t *GRPCTransport) Stop() error {
	if t.server != nil {
		t.server.GracefulStop()
	}
	return nil
}

// RegisterRequestHandler registers handlers for incoming requests
func (t *GRPCTransport) RegisterRequestHandler(handler raft.RequestHandler) error {
	if handler == nil {
		return ErrNilHandler
	}
	t.handler = handler
	return nil
}

// SendVoteRequest sends a vote request to a target node
func (t *GRPCTransport) SendVoteRequest(ctx context.Context, target cluster.Node, req *raft.VoteRequest) (*raft.VoteResponse, error) {
	resp, err := t.sendRPC(&pb.VoteRequest{
		Term:         req.Term,
		CandidateId:  req.CandidateId,
		LastLogIndex: req.LastLogIndex,
		LastLogTerm:  req.LastLogTerm,
	}, target, ctx)
	if err != nil {
		return nil, err
	}

	voteResp := resp.(*pb.VoteResponse)
	return &raft.VoteResponse{
		Term:        voteResp.Term,
		VoteGranted: voteResp.VoteGranted,
	}, nil
}

// SendAppendEntries sends append entries request to a target node
func (t *GRPCTransport) SendAppendEntries(ctx context.Context, target cluster.Node, req *raft.AppendEntriesRequest) (*raft.AppendEntriesResponse, error) {
	entries := make([]*pb.Entry, len(req.Entries))
	for i, e := range req.Entries {
		entries[i] = &pb.Entry{
			Term:  e.Term,
			Index: e.Index,
			Data:  e.Data,
			Type:  e.Type,
		}
	}

	resp, err := t.sendRPC(&pb.AppendEntriesRequest{
		Term:         req.Term,
		LeaderId:     req.LeaderId,
		PrevLogIndex: req.PrevLogIndex,
		PrevLogTerm:  req.PrevLogTerm,
		Entries:      entries,
		LeaderCommit: req.LeaderCommit,
	}, target, ctx)

	if err != nil {
		return nil, err
	}

	appendResp := resp.(*pb.AppendEntriesResponse)

	return &raft.AppendEntriesResponse{
		Id:      appendResp.Id,
		Term:    appendResp.Term,
		Success: appendResp.Success,
	}, nil
}

// SendApplyRequest forwards an apply request to a target node
func (t *GRPCTransport) SendApplyRequest(ctx context.Context, target cluster.Node, req *raft.ApplyRequest) (*raft.ApplyResponse, error) {
	resp, err := t.sendRPC(&pb.ApplyRequest{
		Command: req.Command,
	}, target, ctx)
	if err != nil {
		return nil, err
	}

	applyResp := resp.(*pb.ApplyResponse)
	return &raft.ApplyResponse{
		Result: raft.ApplyResult(applyResp.Result),
	}, nil
}

func (t *GRPCTransport) sendRPC(req proto.Message, target cluster.Node, ctx context.Context) (any, error) {
	var creds credentials.TransportCredentials
	if t.tlsConfig == nil {
		creds = insecure.NewCredentials()
	} else {
		creds = credentials.NewTLS(t.tlsConfig)
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
	}

	if t.dialer != nil {
		opts = append(opts, grpc.WithContextDialer(t.dialer))
	}
	conn, err := grpc.NewClient(target.Addr, opts...)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = conn.Close()
	}()
	c := pb.NewRaftClient(conn)

	var res interface{}
	for i := 0; i < t.maxRetries; i++ {
		switch req := req.(type) {
		case *pb.VoteRequest:
			res, err = c.RequestVote(ctx, req)
		case *pb.AppendEntriesRequest:
			res, err = c.AppendEntry(ctx, req)
		case *pb.ApplyRequest:
			res, err = c.ForwardApply(ctx, req)
		default:
			return nil, ErrInvalidRequestType
		}

		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			break
		default:
			time.Sleep(t.retryDelay)
		}
	}

	return res, err
}

// grpcTransportServer implements the gRPC server for Raft transport
type grpcTransportServer struct {
	pb.UnimplementedRaftServer
	r raft.RequestHandler
}

func (g *grpcTransportServer) ForwardApply(_ context.Context, request *pb.ApplyRequest) (*pb.ApplyResponse, error) {
	resp := g.r.OnForwardApplyRequest(&raft.ApplyRequest{
		Command: request.Command,
	})
	return &pb.ApplyResponse{
		Result: pb.ApplyResult(resp.Result),
	}, nil
}

func (g *grpcTransportServer) RequestVote(_ context.Context, request *pb.VoteRequest) (*pb.VoteResponse, error) {
	resp := g.r.OnRequestVote(&raft.VoteRequest{
		Term:         request.Term,
		CandidateId:  request.CandidateId,
		LastLogIndex: request.LastLogIndex,
		LastLogTerm:  request.LastLogTerm,
	})
	return &pb.VoteResponse{
		Term:        resp.Term,
		VoteGranted: resp.VoteGranted,
	}, nil
}

func (g *grpcTransportServer) AppendEntry(_ context.Context, request *pb.AppendEntriesRequest) (*pb.AppendEntriesResponse, error) {
	req := &raft.AppendEntriesRequest{
		Term:         request.Term,
		LeaderId:     request.LeaderId,
		PrevLogIndex: request.PrevLogIndex,
		PrevLogTerm:  request.PrevLogTerm,
		Entries:      make([]*raft.Entry, len(request.Entries)),
		LeaderCommit: request.LeaderCommit,
	}
	for i, e := range request.Entries {
		req.Entries[i] = &raft.Entry{
			Term:  e.Term,
			Index: e.Index,
			Data:  e.Data,
			Type:  e.Type,
		}
	}

	resp := g.r.OnAppendEntry(req)

	return &pb.AppendEntriesResponse{
		Id:      resp.Id,
		Term:    resp.Term,
		Success: resp.Success,
	}, nil
}

var (
	ErrNoHandlerRegistered = errors.New("no request handler registered")
	ErrNilHandler          = errors.New("nil request handler provided")
	ErrInvalidRequestType  = errors.New("invalid request type")
)

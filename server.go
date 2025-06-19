package raft

import (
	"context"
	"crypto/tls"
	"github.com/Mathew-Estafanous/raft/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"net"
)

type requestHandler interface {
	onAppendEntry(req *pb.AppendEntriesRequest) *pb.AppendEntriesResponse
	onRequestVote(req *pb.VoteRequest) *pb.VoteResponse
	onForwardApplyRequest(req *pb.ApplyRequest) *pb.ApplyResponse
}

type server struct {
	r   requestHandler
	lis net.Listener
	rpc *grpc.Server
}

func newServer(raft *Raft, lis net.Listener, tlsConfig *tls.Config) *server {
	var rpcServer *grpc.Server
	if tlsConfig != nil {
		creds := credentials.NewTLS(tlsConfig)
		rpcServer = grpc.NewServer(grpc.Creds(creds))
	} else {
		rpcServer = grpc.NewServer()
	}
	return &server{
		r:   raft,
		lis: lis,
		rpc: rpcServer,
	}
}

func (s *server) serve() error {
	pb.RegisterRaftServer(s.rpc, gRPCRaftServer{r: s.r})
	err := s.rpc.Serve(s.lis)
	if err != nil {
		return err
	}
	return nil
}

func (s *server) shutdown() {
	s.rpc.Stop()
}

type rpcResp struct {
	resp  interface{}
	error error
}

type gRPCRaftServer struct {
	pb.UnimplementedRaftServer
	r requestHandler
}

func (g gRPCRaftServer) ForwardApply(_ context.Context, request *pb.ApplyRequest) (*pb.ApplyResponse, error) {
	resp := g.r.onForwardApplyRequest(request)
	return resp, nil
}

func (g gRPCRaftServer) RequestVote(_ context.Context, request *pb.VoteRequest) (*pb.VoteResponse, error) {
	resp := g.r.onRequestVote(request)
	return resp, nil
}

func (g gRPCRaftServer) AppendEntry(_ context.Context, request *pb.AppendEntriesRequest) (*pb.AppendEntriesResponse, error) {
	resp := g.r.onAppendEntry(request)
	return resp, nil
}

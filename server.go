package raft

import (
	"context"
	"github.com/Mathew-Estafanous/raft/pb"
	"google.golang.org/grpc"
	"net"
)

type server struct {
	r   *Raft
	lis net.Listener
	rpc *grpc.Server
}

func newServer(raft *Raft, lis net.Listener) *server {
	return &server{
		r:   raft,
		lis: lis,
		rpc: grpc.NewServer(),
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

func toRPCResponse(r interface{}, err error) rpcResp {
	return rpcResp{
		resp:  r,
		error: err,
	}
}

type gRPCRaftServer struct {
	r *Raft
}

func (g gRPCRaftServer) RequestVote(ctx context.Context, request *pb.VoteRequest) (*pb.VoteResponse, error) {
	r := g.r.handleRPC(request)
	if r.error != nil {
		return nil, r.error
	}
	return r.resp.(*pb.VoteResponse), nil
}

func (g gRPCRaftServer) AppendEntry(ctx context.Context, request *pb.AppendEntriesRequest) (*pb.AppendEntriesResponse, error) {
	r := g.r.handleRPC(request)
	if r.error != nil {
		return nil, r.error
	}
	return r.resp.(*pb.AppendEntriesResponse), nil
}

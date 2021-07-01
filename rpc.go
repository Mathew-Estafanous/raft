package raft

import (
	"context"
	"github.com/Mathew-Estafanous/raft/pb"
)

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

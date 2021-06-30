package raft

import (
	"context"
	"github.com/Mathew-Estafanous/raft/pb"
)

type RPCResponse struct {
	resp  interface{}
	error error
}

func ToRPCResponse(r interface{}, err error) RPCResponse {
	return RPCResponse{
		resp:  r,
		error: err,
	}
}

type gRPCRaftServer struct {
	r *Raft
}

func (g gRPCRaftServer) RequestVote(ctx context.Context, request *pb.VoteRequest) (*pb.VoteResponse, error) {
	r := g.r.state.handleRPC(request)
	if r.error != nil {
		return nil, r.error
	}
	return r.resp.(*pb.VoteResponse), nil
}

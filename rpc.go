package raft

import (
	"context"
	"fmt"
	"github.com/Mathew-Estafanous/raft/pb"
	"google.golang.org/grpc"
	"log"
)

func (r *Raft) sendRPC(req interface{}, target Node) rpcResp {
	conn, err := grpc.Dial(target.Addr, grpc.WithInsecure())
	if err != nil {
		return toRPCResponse(nil, err)
	}

	defer func() {
		if err = conn.Close(); err != nil {
			r.logger.Print("Encountered an issue when closing connection: %v", err)
		}
	}()
	c := pb.NewRaftClient(conn)

	var res interface{}
	ctx := context.Background()
	switch req := req.(type) {
	case *pb.VoteRequest:
		res, err = c.RequestVote(ctx, req)
	case *pb.AppendEntriesRequest:
		res, err = c.AppendEntry(ctx, req)
	case *pb.ApplyRequest:
		res, err = c.ForwardApply(ctx, req)
	default:
		log.Fatalf("[BUG] Could not determine RPC request of %v", req)
	}
	return toRPCResponse(res, err)
}

func (r *Raft) handleRPC(req interface{}) rpcResp {
	var rpcErr error
	var resp interface{}

	switch req := req.(type) {
	case *pb.VoteRequest:
		resp = r.onRequestVote(req)
	case *pb.AppendEntriesRequest:
		resp = r.onAppendEntry(req)
	case *pb.ApplyRequest:
		resp = r.onForwardApplyRequest(req)
	default:
		rpcErr = fmt.Errorf("unable to response to rpcResp request")
	}

	return toRPCResponse(resp, rpcErr)
}

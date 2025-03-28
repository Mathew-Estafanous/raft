package raft

import (
	"context"
	"github.com/Mathew-Estafanous/raft/pb"
	"google.golang.org/grpc"
	"log"
)

func sendRPC(req interface{}, target Node) rpcResp {
	conn, err := grpc.Dial(target.Addr, grpc.WithInsecure())
	if err != nil {
		return rpcResp{
			resp:  nil,
			error: err,
		}
	}

	defer func() {
		_ = conn.Close()
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

	return rpcResp{
		resp:  res,
		error: nil,
	}
}

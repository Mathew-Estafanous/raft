package raft

import (
	"context"
	"crypto/tls"
	"github.com/Mathew-Estafanous/raft/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"log"
)

func sendRPC(req interface{}, target Node, config *tls.Config) rpcResp {
	var creds credentials.TransportCredentials
	if config == nil {
		creds = insecure.NewCredentials()
	} else {
		creds = credentials.NewTLS(config)
	}
	conn, err := grpc.NewClient(target.Addr, grpc.WithTransportCredentials(creds))
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
		error: err,
	}
}

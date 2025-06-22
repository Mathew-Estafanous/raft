package raft

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"time"

	"github.com/Mathew-Estafanous/raft/cluster"
	"github.com/Mathew-Estafanous/raft/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var maxRetries = 3
var retryDelay = 40 * time.Millisecond

type Dialer func(context.Context, string) (net.Conn, error)

func sendRPC(req interface{}, target cluster.Node, ctx context.Context, config *tls.Config, dialer Dialer) rpcResp {
	var creds credentials.TransportCredentials
	if config == nil {
		creds = insecure.NewCredentials()
	} else {
		creds = credentials.NewTLS(config)
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
	}

	if dialer != nil {
		opts = append(opts, grpc.WithContextDialer(dialer))
	}
	conn, err := grpc.NewClient(target.Addr, opts...)
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
	for i := 0; i < maxRetries; i++ {
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

		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			break
		default:
			time.Sleep(retryDelay)
		}
	}

	return rpcResp{
		resp:  res,
		error: err,
	}
}

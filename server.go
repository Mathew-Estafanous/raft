package raft

import (
	"github.com/Mathew-Estafanous/raft/pb"
	"google.golang.org/grpc"
	"net"
	"sync"
)

type server struct {
	mu sync.Mutex

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

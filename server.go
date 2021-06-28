package raft

import (
	"fmt"
	"google.golang.org/grpc"
	"net"
)

type server struct {
	r   *Raft
	l   net.Listener
	rpc *grpc.Server
}

func newServer(raft *Raft, lis net.Listener) *server {
	return &server{
		r:   raft,
		l:   lis,
		rpc: grpc.NewServer(),
	}
}

func (s *server) start() error {
	err := s.rpc.Serve(s.l)
	if err != nil {
		return fmt.Errorf("server for raft %d unexpectedly shutdown", s.r.id)
	}
	return nil
}

func (s server) stop() {
	s.rpc.Stop()
}

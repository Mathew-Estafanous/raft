package raft

import (
	"net"
	"net/rpc"
	"sync"
)

type server struct {
	mu sync.Mutex
	wg sync.WaitGroup

	r   *Raft
	lis net.Listener
	rpc *rpc.Server
}

func newServer(raft *Raft, lis net.Listener) *server {
	return &server{
		r:   raft,
		lis: lis,
		rpc: rpc.NewServer(),
	}
}

func (s *server) serve() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		conn, err := s.lis.Accept()
		if err != nil {
			s.r.logger.Println("Listener failed: ", err.Error())
			return
		}
		s.wg.Add(1)
		go func() {
			s.rpc.ServeConn(conn)
			s.wg.Done()
		}()
	}()
}

func (s *server) shutdown() error {
	err := s.lis.Close()
	if err != nil {
		return err
	}
	s.wg.Wait()
	return nil
}

package main

import (
	"github.com/Mathew-Estafanous/raft"
	"log"
	"net"
)

func main() {
	c := raft.NewCluster()

	r1, err := raft.NewRaft(c, 1)
	if err != nil {
		log.Fatalln(err)
	}

	r2, err := raft.NewRaft(c, 2)
	if err != nil {
		log.Fatalln(err)
	}

	closeCh := make(chan bool)
	lis1, err := net.Listen("tcp", ":9000")
	if err != nil {
		log.Fatal(err)
	}
	lis2, err := net.Listen("tcp", ":8000")
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		_ = r1.Serve(lis1)
		closeCh <- true
	}()

	go func() {
		_ = r2.Serve(lis2)
		closeCh <- true
	}()

	<- closeCh
}

package main

import (
	"github.com/Mathew-Estafanous/raft"
	"log"
	"os"
	"os/signal"
	"syscall"
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

	r3, err := raft.NewRaft(c, 3)
	if err != nil {
		log.Fatalln(err)
	}

	closeCh := make(chan os.Signal)
	signal.Notify(closeCh, os.Interrupt, syscall.SIGTERM, syscall.SIGTERM)
	err = r1.ListenAndServe(":9000")
	if err != nil {
		log.Fatalln(err)
	}
	err = r2.ListenAndServe(":8000")
	if err != nil {
		log.Fatalln(err)
	}
	err = r3.ListenAndServe(":7000")
	if err != nil {
		log.Fatalln(err)
	}

	<-closeCh
	log.Println("SHUTDOWN!")
}

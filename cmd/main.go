package main

import (
	"github.com/Mathew-Estafanous/raft"
	"log"
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
	err = r1.ListenAndServe(":9000")
	if err != nil {
		log.Fatalln(err)
	}
	err = r2.ListenAndServe(":8000")
	if err != nil {
		log.Fatalln(err)
	}

	<-closeCh
}

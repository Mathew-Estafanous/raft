package main

import (
	"fmt"
	"github.com/Mathew-Estafanous/raft"
	"log"
	"os"
	"sync"
)

type someFSM struct{}

func (s someFSM) Apply(data []byte) error {
	log.Println(string(data))
	return nil
}

func main() {
	c := raft.NewCluster()

	r1, err := raft.New(c, 1, new(someFSM))
	if err != nil {
		log.Fatalln(err)
	}

	r2, err := raft.New(c, 2, new(someFSM))
	if err != nil {
		log.Fatalln(err)
	}

	r3, err := raft.New(c, 3, new(someFSM))
	if err != nil {
		log.Fatalln(err)
	}

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		if err = r1.ListenAndServe(":8000"); err != nil {
			log.Println(err)
		}
		wg.Done()
	}()
	go func() {
		if err = r2.ListenAndServe(":8001"); err != nil {
			log.Println(err)
		}
		wg.Done()
	}()
	go func() {
		if err = r3.ListenAndServe(":8002"); err != nil {
			log.Println(err)
		}
		wg.Done()
	}()

	raftM := map[int]*raft.Raft{
		1: r1,
		2: r2,
		3: r3,
	}

	var r int
	var cmd string
	for {
		fmt.Println("Raft | CMD")
		_, err = fmt.Fscanln(os.Stdin, &r, &cmd)
		if err != nil {
			log.Fatal(err)
		}
		if r == -1 {
			break
		}

		t := raftM[r].Apply([]byte(cmd))
		fmt.Println(t.Error())
	}

	wg.Wait()
	log.Println("Raft cluster simulation shutdown.")
}

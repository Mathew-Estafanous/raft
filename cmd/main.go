package main

import (
	"fmt"
	"github.com/Mathew-Estafanous/raft"
	"log"
	"os"
	"strconv"
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
	//go serve(r1, ":8000", &wg)
	go serve(r2, ":8001", &wg)
	go serve(r3, ":8002", &wg)

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

		if cmd == "sh" {
			raftM[r].Shutdown()
		} else if cmd == "st" {
			wg.Add(1)
			go serve(raftM[r], ":" + strconv.Itoa(7999 + r), &wg)
		} else {
			t := raftM[r].Apply([]byte(cmd))
			fmt.Println(t.Error())
		}
	}

	wg.Wait()
	log.Println("Raft cluster simulation shutdown.")
}

func serve(r *raft.Raft, port string, wg *sync.WaitGroup) {
	if err := r.ListenAndServe(port); err != nil {
		log.Println(err)
	}
	wg.Done()
}

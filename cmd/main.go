package main

import (
	"github.com/Mathew-Estafanous/raft"
	"log"
	"sync"
)

type someFSM struct{}

func (s someFSM) Apply(data []byte) error {
	log.Println(data)
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
	wg.Wait()
	log.Println("Raft cluster simulation shutdown.")
}

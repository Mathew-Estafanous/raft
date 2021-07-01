package main

import (
	"github.com/Mathew-Estafanous/raft"
	"log"
	"sync"
)

func main() {
	c := raft.NewCluster()

	r1, err := raft.New(c, 1)
	if err != nil {
		log.Fatalln(err)
	}

	r2, err := raft.New(c, 2)
	if err != nil {
		log.Fatalln(err)
	}

	r3, err := raft.New(c, 3)
	if err != nil {
		log.Fatalln(err)
	}

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		if err = r1.ListenAndServe(":9000"); err != nil {
			log.Println(err)
		}
		wg.Done()
	}()
	go func() {
		if err = r2.ListenAndServe(":8000"); err != nil {
			log.Println(err)
		}
		wg.Done()
	}()
	go func() {
		if err = r3.ListenAndServe(":7000"); err != nil {
			log.Println(err)
		}
		wg.Done()
	}()
	wg.Wait()
	log.Println("Raft cluster simulation shutdown.")
}

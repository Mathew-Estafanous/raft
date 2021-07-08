package main

import (
	"github.com/Mathew-Estafanous/raft"
	"log"
	"net/http"
	"strconv"
	"sync"
)

func main() {
	c := raft.NewCluster()
	var wg sync.WaitGroup

	wg.Add(3)
	go makeAndRunKV(1, c, &wg)
	go makeAndRunKV(2, c, &wg)
	go makeAndRunKV(3, c, &wg)

	wg.Wait()
	log.Println("Raft cluster simulation shutdown.")
}

func makeAndRunKV(id uint64, c *raft.Cluster, wg *sync.WaitGroup) {
	kv := NewStore()
	r, err := raft.New(c, id, kv)
	kv.r = r
	if err != nil {
		log.Fatalln(err)
	}

	raftPort := ":" + strconv.Itoa(int(6000 + id))
	go func() {
		if err := r.ListenAndServe(raftPort); err != nil {
			log.Println(err)
		}
	}()

	kvPort := ":" + strconv.Itoa(int(8000 + id))
	if err = http.ListenAndServe(kvPort, kv); err != nil {
		log.Println(err)
	}
	wg.Done()
}

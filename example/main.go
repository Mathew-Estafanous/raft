// This is meant as a very simple example of how this raft implementation can be
// used to create a distributed KV store database.
//
// The code here is not meant to be used in any serious production
// environment and just showcases the capabilities of this raft library.
package main

import (
	"github.com/Mathew-Estafanous/raft"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

func main() {
	c := raft.NewCluster()
	var wg sync.WaitGroup

	wg.Add(3)
	go makeAndRunKV(1, c, createMemStore(1), &wg)
	go makeAndRunKV(2, c, createMemStore(2), &wg)
	go makeAndRunKV(3, c, createMemStore(3), &wg)

	time.Sleep(45 * time.Second)
	go makeAndRunKV(4, c, createMemStore(4), &wg)

	wg.Wait()
	log.Println("Raft cluster simulation shutdown.")
}

func makeAndRunKV(id uint64, c *raft.Cluster, mem *raft.InMemStore, wg *sync.WaitGroup) {
	kv := NewStore()
	r, err := raft.New(c, id, raft.SlowRaftOpts, kv, mem, mem)
	kv.r = r
	if err != nil {
		log.Fatalln(err)
	}

	raftPort := ":" + strconv.Itoa(int(6000+id))
	go func() {
		if err := r.ListenAndServe(raftPort); err != nil {
			log.Println(err)
		}
	}()

	kvPort := ":" + strconv.Itoa(int(8000+id))
	if err = http.ListenAndServe(kvPort, kv); err != nil {
		log.Println(err)
	}
	wg.Done()
}

func createMemStore(profile int) *raft.InMemStore {
	mem := raft.NewMemStore()
	var logs []*raft.Log
	var term int
	if profile == 1 {
		logs = []*raft.Log{
			{
				Type: raft.Entry,
				Index: 0,
				Term:  1,
				Cmd:   []byte("my friends"),
			},
			{
				Type: raft.Entry,
				Index: 1,
				Term:  1,
				Cmd:   []byte("my bro"),
			},
			{
				Type: raft.Entry,
				Index: 2,
				Term:  2,
				Cmd:   []byte("mat amazing"),
			},
		}
		term = 2
	} else if profile == 2 {
		logs = []*raft.Log{
			{
				Type: raft.Entry,
				Index: 0,
				Term:  1,
				Cmd:   []byte("my friends"),
			},
			{
				Type: raft.Entry,
				Index: 1,
				Term:  1,
				Cmd:   []byte("my bro"),
			},
			{
				Type: raft.Entry,
				Index: 2,
				Term:  2,
				Cmd:   []byte("mat amazing"),
			},
			{
				Type: raft.Entry,
				Index: 3,
				Term:  2,
				Cmd:   []byte("hello world"),
			},
		}
		term = 2
	} else if profile == 3 {
		logs = []*raft.Log{
			{
				Type: raft.Entry,
				Index: 0,
				Term:  1,
				Cmd:   []byte("my friends"),
			},
		}
		term = 1
	}
	mem.AppendLogs(logs)
	mem.Set([]byte("currentTerm"), []byte(strconv.Itoa(term)))
	return mem
}

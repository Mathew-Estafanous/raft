// This is meant as a very simple example of how this raft implementation can be
// used to create a distributed KV store database.
//
// The code here is not meant to be used in any serious production
// environment and just showcases the capabilities of this raft library.
package main

import (
	"fmt"
	"github.com/Mathew-Estafanous/raft"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
)

// [exe] <MemberPort> <ID> <Address* (of another node in the cluster)>
func main() {
	memPort, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatalln(err)
	}
	id, err := strconv.Atoi(os.Args[2])
	if err != nil {
		log.Fatalln(err)
	}

	ip := getLocalIP()
	if ip == "" {
		ip = "127.0.0.1"
	}
	raftAddr := fmt.Sprintf("%v:%v", ip, strconv.Itoa(6000+id))

	c, err := raft.NewDynamicCluster(ip, uint16(memPort), raft.Node{ID: uint64(id), Addr: raftAddr})
	if err != nil {
		log.Fatalln(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	if len(os.Args) >= 4 {
		if err = c.Join(os.Args[3]); err != nil {
			log.Fatalln(err)
		}
	}
	go makeAndRunKV(raftAddr, uint64(id), c, createMemStore(id), &wg)
	wg.Wait()
	log.Println("Raft cluster simulation shutdown.")
}

func makeAndRunKV(raftAddr string, id uint64, c raft.Cluster, mem *raft.InMemStore, wg *sync.WaitGroup) {
	kv := NewStore()
	r, err := raft.New(c, id, raft.SlowOpts, kv, mem, mem)
	kv.r = r
	if err != nil {
		log.Fatalln(err)
	}

	go func() {
		if err := r.ListenAndServe(raftAddr); err != nil {
			log.Println(err)
		}
	}()

	kvPort := ":" + strconv.Itoa(int(8000+id))
	if err = http.ListenAndServe(kvPort, &kvHandler{kv}); err != nil {
		log.Println(err)
	}
	wg.Done()
}

func createMemStore(profile int) *raft.InMemStore {
	mem := raft.NewMemStore()
	logs := make([]*raft.Log, 0)
	var term int
	if profile == 1 {
		logs = []*raft.Log{
			{
				Type:  raft.Entry,
				Index: 0,
				Term:  1,
				Cmd:   []byte("my friends"),
			},
			{
				Type:  raft.Entry,
				Index: 1,
				Term:  1,
				Cmd:   []byte("my bro"),
			},
			{
				Type:  raft.Entry,
				Index: 2,
				Term:  2,
				Cmd:   []byte("mat amazing"),
			},
		}
		term = 2
	} else if profile == 2 {
		logs = []*raft.Log{
			{
				Type:  raft.Entry,
				Index: 0,
				Term:  1,
				Cmd:   []byte("my friends"),
			},
			{
				Type:  raft.Entry,
				Index: 1,
				Term:  1,
				Cmd:   []byte("my bro"),
			},
			{
				Type:  raft.Entry,
				Index: 2,
				Term:  2,
				Cmd:   []byte("mat amazing"),
			},
			{
				Type:  raft.Entry,
				Index: 3,
				Term:  2,
				Cmd:   []byte("hello world"),
			},
		}
		term = 2
	} else if profile >= 3 {
		logs = []*raft.Log{
			{
				Type:  raft.Entry,
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

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

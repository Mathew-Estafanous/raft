// This is meant as a very simple example of how this raft implementation can be
// used to create a distributed KV store database.
//
// The code here is not meant to be used in any serious production
// environment and just showcases the capabilities of this raft library.
package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/Mathew-Estafanous/raft"
	"github.com/Mathew-Estafanous/raft/cluster"
	"github.com/Mathew-Estafanous/raft/store"
	"github.com/Mathew-Estafanous/raft/transport"
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

	c, err := cluster.NewDynamicCluster(ip, uint16(memPort), cluster.Node{ID: uint64(id), Addr: raftAddr})
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

	raftStore, err := store.NewBoltStore(fmt.Sprintf(".data/raft%v.db", id))
	if err != nil {
		log.Fatalln(err)
	}
	go makeAndRunKV(raftAddr, uint64(id), c, raftStore, &wg)
	wg.Wait()
	log.Println("Raft cluster simulation shutdown.")
}

func makeAndRunKV(raftAddr string, id uint64, c cluster.Cluster, raftStore *store.BoltStore, wg *sync.WaitGroup) {
	kv := NewStore()

	list, err := net.Listen("tcp", raftAddr)
	grpcTransport := transport.NewGRPCTransport(list, nil)

	r, err := raft.New(c, id, raft.DefaultOpts, kv, raftStore, raftStore, grpcTransport)
	kv.r = r
	if err != nil {
		log.Fatalln(err)
	}

	go func() {
		if err := r.Serve(); err != nil {
			log.Println(err)
		}
	}()

	kvPort := ":" + strconv.Itoa(int(8000+id))
	if err = http.ListenAndServe(kvPort, &kvHandler{kv}); err != nil {
		log.Println(err)
	}
	wg.Done()
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

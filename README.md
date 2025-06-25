# Raft
[![Go Report Card](https://goreportcard.com/badge/github.com/Mathew-Estafanous/raft)](https://goreportcard.com/report/github.com/Mathew-Estafanous/raft)
[![GoDoc](https://godoc.org/github.com/Mathew-Estafanous/raft?status.svg)](https://pkg.go.dev/github.com/Mathew-Estafanous/raft)
---
Raft is a well known [consensus](https://en.wikipedia.org/wiki/Consensus_(computer_science)) algorithm 
that is used in distributed systems where fault-tolerance and data consensus is important. This is a working 
implementation of that algorithm that enables clients to inject their own systems alongside the raft. Whether 
it be a key-value store or something more complicated.

At the center of the development of this project was the [raft paper](https://web.stanford.edu/~ouster/cgi-bin/papers/raft-atc14)
which thoroughly describes the intricacies of the algorithm, and it's many sub-problems. Spending time researching
and getting a solid understanding of how each problem should be solved prior to coding the solution.

**NOTE:** This implementation is purely for educational purposes so that I could continue learning more about 
distributed systems. This is not intended for production use. If raft is required for production, then it's 
encouraged that [Hashicorp's](https://github.com/hashicorp/raft) library is used instead.

## How It's Used?
This raft implementation is designed with flexibility in mind. Providing a platform to inject your own finite state
machine and your own persistence storage. 

In the [example](https://github.com/Mathew-Estafanous/raft/tree/main/example) use-case, we create a Key-Value store
that implements the [FSM](https://pkg.go.dev/github.com/Mathew-Estafanous/raft#FSM) interface. Now whenever the raft
node needs to apply a task to the FSM, it will apply those tasks through the provided API.
```go
// KvStore type implements all the methods required for the FSM.
type KvStore struct {
	r    *raft.Raft
	data map[string]string
}

func (k *KvStore) Apply(data []byte) error {
	// Apply the command to the FSM
	return nil
}

func (k *KvStore) Snapshot() ([]byte, error) {
	// Create a snapshot of the FSM state
	return nil, nil
}

func (k *KvStore) Restore(cmd []byte) error {
	// Restore the FSM state from a snapshot
	return nil
}
```

The client also needs to create their own implementation of the [LogStore](https://pkg.go.dev/github.com/Mathew-Estafanous/raft#LogStore) 
and [StableStore](https://pkg.go.dev/github.com/Mathew-Estafanous/raft#StableStore). You can use the provided in-memory store for testing,
but for production use, consider using the BoltDB implementation in the store package.
```go
// For testing
memStore := raft.NewMemStore()

// For production
boltStore, err := store.NewBoltStore("/path/to/data")
if err != nil {
    log.Fatalln(err)
}
defer boltStore.Close()
```

All raft nodes are part of a cluster. A cluster is a group of nodes and their associated addresses. There are two types of clusters:

1. **Static Cluster** - A fixed set of nodes that are known from the start. Initialized using a JSON configuration file:
```go
f, err := os.Open("config.json")
if err != nil {
    log.Fatalln(err)
}
c, err := cluster.NewClusterWithConfig(f)
if err != nil {
    log.Fatalln(err)
}
```

2. **Dynamic Cluster** - Allows nodes to join and leave the cluster at runtime:
```go
c, err := cluster.NewDynamicCluster(ip, uint16(memPort), cluster.Node{ID: uint64(id), Addr: raftAddr})
if err != nil {
    log.Fatalln(err)
}

// Join an existing cluster
if err = c.Join(otherNodeAddr); err != nil {
    log.Fatalln(err)
}
```

Once all of the dependencies are initialized, we can now create the raft node and start serving it on the cluster.
```go
r, err := raft.New(c, id, raft.SlowOpts, fsm, boltStore, boltStore)
if err != nil {
    log.Fatalln(err)
}
go func() {
    if err := r.ListenAndServe(raftAddr); err != nil {
        log.Println(err)
    }
}()

// Apply a command to the FSM through the raft node
task := r.Apply([]byte("command"))
if err := task.Error(); err != nil {
    log.Println(err)
}

// Regular operation of the application as the raft node runs in a different goroutine.
```

## In Action (Demo)
At initial startup, all raft nodes will start their servers using a defined configuration file. In this example, there are
three servers with IDs of 1-3. This startup demo shows how all three servers are capable of choosing a leader though an election.

![raft-startup](https://user-images.githubusercontent.com/56979977/127257133-3f888946-6ef7-4bf7-a495-dc965c4adab2.gif)

A major part of the raft consensus algorithm is the ability for the majority of nodes to safely persist the same
data on their finite state machines. This demo shows how the leader (when given a request), replicates the log & state across all
other nodes.

![raft-data-populate](https://user-images.githubusercontent.com/56979977/127257693-03ec9b7c-f9e8-4756-96be-0728f95e92ab.gif)

## Installation

To use this library in your Go project:

```bash
go get github.com/Mathew-Estafanous/raft
```

This library requires Go 1.23 or later.

## Storage Options

This implementation provides multiple storage options:

1. **In-Memory Store** - Provided in the main package, useful for testing but not for production use.
   ```go
   memStore := raft.NewMemStore()
   ```

2. **BoltDB Store** - A persistent storage implementation using BoltDB (bbolt), suitable for production use.
   ```go
   boltStore, err := store.NewBoltStore("/path/to/data")
   if err != nil {
       log.Fatalln(err)
   }
   defer boltStore.Close()
   ```

## Contributing

Contributions are welcome! Feel free to open issues or submit pull requests.

## Connect & Contact
**Email** - mathewestafanous13@gmail.com

**Website** - https://mathewestafanous.xyz

**GitHub** - https://github.com/Mathew-Estafanous

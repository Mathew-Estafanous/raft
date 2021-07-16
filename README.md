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

## Features
List of features that have been developed.
- [X] Raft cluster leader elections.
- [X] Log committing & replication.
- [X] Fault tolerance of N/2 failures. (With N being total # of nodes.)
- [X] Extendable log/stable store interface that lets the client define how they want the
data to be persistently stored. (An In-Memory implementation is provided for testing.)

These are a few features that are expected to be implemented in the future.
- [ ] Log snapshots/compaction 
- [ ] Membership Changes


## Connect & Contact
**Email** - mathewestafanous13@gmail.com

**Website** - https://mathewestafanous.com

**GitHub** - https://github.com/Mathew-Estafanous
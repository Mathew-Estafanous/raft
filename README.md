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

## In Action (Demo)
At initial startup, all raft nodes will start their servers using a defined configuration file. In this example, there are
three servers with IDs of 1-3. This startup demo shows how all three servers are capable of choosing a leader though an election.

![raft-startup](https://user-images.githubusercontent.com/56979977/127257133-3f888946-6ef7-4bf7-a495-dc965c4adab2.gif)

A major part of the raft consensus algorithm is the ability for the majority of nodes to safely persist the same
data on their finite state machines. This demo shows how the leader (when given a request), replicates the log & state across all
other nodes.

![raft-data-populate](https://user-images.githubusercontent.com/56979977/127257693-03ec9b7c-f9e8-4756-96be-0728f95e92ab.gif)

## Features
List of features that have been developed.
- [X] Raft cluster leader elections.
- [X] Log committing & replication.
- [X] Fault tolerance of N/2 failures. (With N being total # of nodes.)
- [X] Extendable log/stable store interface that lets the client define how they want the
data to be persistently stored. (An In-Memory implementation is provided for testing.)
- [X] Log snapshots which compact a given number of logs into one log entry. Enabling state to
be saved while also limiting the length of the log entries.

These are a few features that are expected to be implemented in the future.
- [ ] Membership Changes


## Connect & Contact
**Email** - mathewestafanous13@gmail.com

**Website** - https://mathewestafanous.com

**GitHub** - https://github.com/Mathew-Estafanous

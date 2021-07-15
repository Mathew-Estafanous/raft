# Raft
[![Go Report Card](https://goreportcard.com/badge/github.com/Mathew-Estafanous/raft)](https://goreportcard.com/report/github.com/Mathew-Estafanous/raft)
[![GoDoc](https://godoc.org/github.com/Mathew-Estafanous/raft?status.svg)](https://pkg.go.dev/github.com/Mathew-Estafanous/raft)
---
Raft is a well known [consensus](https://en.wikipedia.org/wiki/Consensus_(computer_science)) algorithm 
that is used in distributed systems where fault-tolerance and data consensus is important. This is a working 
implementation of that algorithm that enables clients to inject their own systems alongside the raft. Whether 
it be a key-value store or something more complicated.

**NOTE:** This implementation is purely for educational purposes so that I could continue learning more about 
distributed systems. This is not intended for production use. If raft is required for production, then it's 
encouraged that [Hashicorp's](https://github.com/hashicorp/raft) library is used instead.

## TODO
- [ ] Initialize app structure and required packages, etc.
- [ ] Work on Raft cluster election, including Candidate, Leader and Followers.
- [ ] Raft consensus using logs.
- [ ] Persistence of vital state information.

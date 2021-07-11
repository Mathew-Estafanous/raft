package raft

import (
	"github.com/Mathew-Estafanous/raft/pb"
	"time"
)

type candidate struct {
	*Raft
	electionTimer *time.Timer
	votesNeeded   int
	voteCh        chan rpcResp
}

func (c *candidate) getType() raftState {
	return Candidate
}

func (c *candidate) runState() {
	c.mu.Lock()
	c.electionTimer.Reset(c.cluster.randElectTime())
	c.setStableStore(keyCurrentTerm, c.fromStableStore(keyCurrentTerm)+1)
	c.logger.Printf("Candidate started election for term %v.", c.fromStableStore(keyCurrentTerm))
	c.mu.Unlock()

	// Run election for candidate by sending request votes to other nodes.
	c.sendVoteRequests()

	for c.getState().getType() == Candidate {
		select {
		case <-c.electionTimer.C:
			c.logger.Printf("Election has failed for term %d", c.fromStableStore(keyCurrentTerm))
			return
		case v := <-c.voteCh:
			if v.error != nil {
				c.logger.Printf("A vote request has failed: %v", v.error)
				break
			}
			vote := v.resp.(*pb.VoteResponse)

			c.handleVoteResponse(vote)
		case t := <-c.applyCh:
			t.respond(ErrNotLeader)
		case <-c.shutdownCh:
			return
		}
	}
}

// sendVoteRequests will initialize and send the vote requests to other nodes
// in the Cluster and return results in a vote channel.
func (c *candidate) sendVoteRequests() {
	c.voteCh = make(chan rpcResp, len(c.cluster.Nodes))
	c.votesNeeded = c.cluster.quorum() - 1
	c.setStableStore(keyVotedFor, c.id)
	req := &pb.VoteRequest{
		Term:         c.fromStableStore(keyCurrentTerm),
		CandidateId:  c.id,
		LastLogIndex: c.log.LastIndex(),
		LastLogTerm:  c.log.LastTerm(),
	}

	for _, v := range c.cluster.Nodes {
		if v.ID == c.id {
			continue
		}

		// Make RPC request in a separate goroutine to prevent blocking operations.
		go func(n node) {
			res := c.sendRPC(req, n)
			c.voteCh <- res
		}(v)
	}
}

func (c *candidate) handleVoteResponse(vote *pb.VoteResponse) {
	// If term of peer is greater then go back to follower
	// and update current term to the peer's term.
	if vote.Term > c.fromStableStore(keyCurrentTerm) {
		c.logger.Println("Demoting since peer's term is greater than current term")
		c.setStableStore(keyCurrentTerm, vote.Term)
		c.setState(Follower)
		return
	}

	if vote.VoteGranted {
		c.votesNeeded--
		// Check if the total votes needed has been reached. If so
		// then election has passed and candidate is now the leader.
		if c.votesNeeded == 0 {
			c.setState(Leader)
		}
	}
}

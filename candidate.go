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

func (c *candidate) getType() StateType {
	return Candidate
}

func (c *candidate) runState() {
	c.mu.Lock()
	c.electionTimer.Reset(c.cluster.randElectTime())
	c.currentTerm++
	c.logger.Printf("Candidate started election for term %v.", c.currentTerm)
	c.mu.Unlock()

	// Run election for candidate by sending request votes to other nodes.
	c.sendVoteRequests()

	for c.getState().getType() == Candidate {
		select {
		case <-c.electionTimer.C:
			c.logger.Printf("Election has failed for term %d", c.currentTerm)
			return
		case v := <-c.voteCh:
			if v.error != nil {
				c.logger.Printf("A vote request has failed: %v", v.error)
				break
			}
			vote := v.resp.(*pb.VoteResponse)

			// If term of peer is greater then go back to follower
			// and update current term to the peer's term.
			if vote.Term > c.currentTerm {
				c.logger.Println("Demoting since peer's term is greater than current term")
				c.mu.Lock()
				c.currentTerm = vote.Term
				c.mu.Unlock()
				c.setState(Follower)
				break
			}

			if vote.VoteGranted {
				c.votesNeeded--
				// Check if the total votes needed has been reached. If so
				// then election has passed and candidate is now the leader.
				if c.votesNeeded == 0 {
					c.setState(Leader)
				}
			}
		case <-c.shutdownCh:
			return
		}
	}
}

// sendVoteRequests will initialize and send the vote requests to other nodes
// in the cluster and return results in a vote channel.
func (c *candidate) sendVoteRequests() {
	c.voteCh = make(chan rpcResp, len(c.cluster.Nodes))
	c.votesNeeded = c.cluster.quorum() - 1
	c.voteTerm = int64(c.currentTerm)
	c.votedFor = c.id
	req := &pb.VoteRequest{
		Term:        c.currentTerm,
		CandidateId: c.id,
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

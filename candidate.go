package raft

import (
	"fmt"
	"github.com/Mathew-Estafanous/raft/pb"
)

func (r *Raft) runCandidateState() {
	r.electionTimer.Reset(r.randElectTime())
	r.setStableStore(keyCurrentTerm, r.fromStableStore(keyCurrentTerm)+1)
	r.logger.Printf("Candidate started election for term %v.", r.fromStableStore(keyCurrentTerm))

	// Run election for candidate by sending request votes to other nodes.
	r.sendVoteRequests()

	for r.getState() == Candidate {
		select {
		case <-r.electionTimer.C:
			r.logger.Printf("Election has failed for term %d", r.fromStableStore(keyCurrentTerm))
			return
		case v := <-r.voteCh:
			if v.error != nil {
				r.logger.Printf("A vote request has failed: %v", v.error)
				break
			}
			vote := v.resp.(*pb.VoteResponse)

			r.handleVoteResponse(vote)
		case t := <-r.applyCh:
			t.respond(fmt.Errorf("no leader assigned for term, try again later"))
		case <-r.shutdownCh:
			return
		}
	}
}

// sendVoteRequests will initialize and send the vote requests to other nodes
// in the Cluster and return results in a vote channel.
func (r *Raft) sendVoteRequests() {
	r.voteCh = make(chan rpcResp, len(r.cluster.AllNodes()))
	r.votesNeeded = r.cluster.Quorum() - 1
	r.setStableStore(keyVotedFor, r.id)
	req := &pb.VoteRequest{
		Term:         r.fromStableStore(keyCurrentTerm),
		CandidateId:  r.id,
		LastLogIndex: r.log.LastIndex(),
		LastLogTerm:  r.log.LastTerm(),
	}

	for _, v := range r.cluster.AllNodes() {
		if v.ID == r.id {
			continue
		}

		// Make RPC request in a separate goroutine to prevent blocking operations.
		go func(n Node) {
			res := sendRPC(req, n)
			r.voteCh <- res
		}(v)
	}
}

func (r *Raft) handleVoteResponse(vote *pb.VoteResponse) {
	// If term of peer is greater than go back to follower
	// and update current term to the peer's term.
	if vote.Term > r.fromStableStore(keyCurrentTerm) {
		r.logger.Println("Demoting since peer's term is greater than current term")
		r.setStableStore(keyCurrentTerm, vote.Term)
		r.setState(Follower)
		return
	}

	if vote.VoteGranted {
		r.votesNeeded--
		// Check if the total votes needed has been reached. If so
		// then election has passed and candidate is now the leader.
		if r.votesNeeded == 0 {
			r.setState(Leader)
		}
	}
}

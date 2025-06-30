package raft

import (
	"log/slog"

	"github.com/Mathew-Estafanous/raft/cluster"
	"golang.org/x/net/context"
)

func (r *Raft) runCandidateState() {
	electCtx, cancel := context.WithTimeout(context.Background(), r.opts.MaxElectionTimout)
	defer cancel()

	r.setStableStore(keyCurrentTerm, r.fromStableStore(keyCurrentTerm)+1)
	candidateLogger := r.logger.With(slog.Uint64("term", r.fromStableStore(keyCurrentTerm)))
	candidateLogger.Info("Candidate started election")

	r.sendVoteRequests(electCtx)

	for r.getState() == Candidate {
		select {
		case <-electCtx.Done():
			candidateLogger.InfoContext(electCtx, "Election failed. Demoting to follower.")
			r.setState(Follower)
		case v := <-r.voteCh:
			if v.error != nil {
				candidateLogger.Warn("Vote request failed",
					slog.String("error", v.error.Error()),
				)
				break
			}

			r.handleVoteResponse(v.resp)
		case <-r.stateCh:
			break
		case <-r.shutdownCh:
			cancel()
			return
		}
	}
}

// sendVoteRequests will initialize and send the vote requests to other nodes
// in the Cluster and return results in a vote channel.
func (r *Raft) sendVoteRequests(ctx context.Context) {
	nodes := r.cluster.AllNodes()
	r.voteCh = make(chan rpcResponse[*VoteResponse], len(nodes))
	r.votesNeeded = r.cluster.Quorum() - 1
	r.setStableStore(keyVotedFor, r.id)
	req := &VoteRequest{
		Term:         r.fromStableStore(keyCurrentTerm),
		CandidateId:  r.id,
		LastLogIndex: r.log.LastIndex(),
		LastLogTerm:  r.log.LastTerm(),
	}

	for _, v := range nodes {
		if v.ID == r.id {
			continue
		}

		// Make RPC request in a separate goroutine to prevent blocking operations.
		go func(ctx context.Context, n cluster.Node, voteCh chan rpcResponse[*VoteResponse], req *VoteRequest) {
			resp, err := r.transport.SendVoteRequest(ctx, n, req)
			res := rpcResponse[*VoteResponse]{
				resp:  resp,
				error: err,
			}
			select {
			case <-ctx.Done():
				return
			case voteCh <- res:
			}
		}(ctx, v, r.voteCh, req)
	}
}

func (r *Raft) handleVoteResponse(vote *VoteResponse) {
	// If term of peer is greater than go back to follower
	// and update current term to the peer's term.
	if vote.Term > r.fromStableStore(keyCurrentTerm) {
		r.logger.Info("Demoting since peer's term is greater than current term")
		r.setStableStore(keyCurrentTerm, vote.Term)
		r.setState(Follower)
		return
	}

	if vote.VoteGranted {
		r.votesNeeded--
		// Check if the total votes needed have been reached. If so, then the
		// election has passed and the candidate is now the leader.
		if r.votesNeeded == 0 {
			r.setState(Leader)
		}
	}
}

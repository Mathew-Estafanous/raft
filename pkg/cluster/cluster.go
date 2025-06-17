package cluster

// Cluster keeps track of all other nodes and their addresses.
// It also holds agreed upon constants such as heart beat time and election timeout.
type Cluster interface {
	GetNode(id uint64) (Node, error)
	AllNodes() map[uint64]Node
	Quorum() int
}

type Node struct {
	// An ID that uniquely identifies the raft in the Cluster.
	ID uint64 `json:"id"`

	// Address of the node, that other rafts can contact.
	Addr string `json:"addr"`
}

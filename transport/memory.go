package transport

import (
	"context"
	"errors"
	"sync"

	"github.com/Mathew-Estafanous/raft"
	"github.com/Mathew-Estafanous/raft/cluster"
)

// Registry manages a collection of in-memory transports
type Registry struct {
	transports map[string]*MemoryTransport
	mu         sync.RWMutex
}

// NewRegistry creates a new registry for in-memory transports
func NewRegistry() *Registry {
	return &Registry{
		transports: make(map[string]*MemoryTransport),
	}
}

// MemoryTransport implements the Transport interface for in-memory communication
// This is primarily useful for testing purposes
type MemoryTransport struct {
	addr     string
	handler  raft.RequestHandler
	running  bool
	mu       sync.RWMutex
	registry *Registry
}

// NewMemoryTransport creates a new in-memory transport with a custom registry
func NewMemoryTransport(addr string, registry *Registry) *MemoryTransport {
	return &MemoryTransport{
		addr:     addr,
		registry: registry,
	}
}

// Start initializes and starts the transport layer
func (t *MemoryTransport) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.handler == nil {
		return ErrNoHandlerRegistered
	}

	t.registry.mu.Lock()
	defer t.registry.mu.Unlock()

	if _, exists := t.registry.transports[t.addr]; exists {
		return errors.New("memory transport already registered with this address")
	}

	t.registry.transports[t.addr] = t
	t.running = true
	return nil
}

// Stop gracefully shuts down the transport layer
func (t *MemoryTransport) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.running {
		return nil
	}

	t.registry.mu.Lock()
	defer t.registry.mu.Unlock()

	delete(t.registry.transports, t.addr)
	t.running = false
	return nil
}

// RegisterRequestHandler registers handlers for incoming requests
func (t *MemoryTransport) RegisterRequestHandler(handler raft.RequestHandler) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if handler == nil {
		return ErrNilHandler
	}
	t.handler = handler
	return nil
}

// SendVoteRequest sends a vote request to a target node
func (t *MemoryTransport) SendVoteRequest(ctx context.Context, target cluster.Node, req *raft.VoteRequest) (*raft.VoteResponse, error) {
	targetTransport, err := t.getTargetTransport(target.Addr)
	if err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return targetTransport.handler.OnRequestVote(req), nil
	}
}

// SendAppendEntries sends append entries request to a target node
func (t *MemoryTransport) SendAppendEntries(ctx context.Context, target cluster.Node, req *raft.AppendEntriesRequest) (*raft.AppendEntriesResponse, error) {
	targetTransport, err := t.getTargetTransport(target.Addr)
	if err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return targetTransport.handler.OnAppendEntry(req), nil
	}
}

// SendApplyRequest forwards an apply request to a target node
func (t *MemoryTransport) SendApplyRequest(ctx context.Context, target cluster.Node, req *raft.ApplyRequest) (*raft.ApplyResponse, error) {
	targetTransport, err := t.getTargetTransport(target.Addr)
	if err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return targetTransport.handler.OnForwardApplyRequest(req), nil
	}
}

// getTargetTransport retrieves the target transport from the registry
func (t *MemoryTransport) getTargetTransport(addr string) (*MemoryTransport, error) {
	t.registry.mu.RLock()
	defer t.registry.mu.RUnlock()

	targetTransport, exists := t.registry.transports[addr]
	if !exists {
		return nil, errors.New("target node not found in memory transport registry")
	}

	targetTransport.mu.RLock()
	defer targetTransport.mu.RUnlock()

	if !targetTransport.running {
		return nil, errors.New("target node transport is not running")
	}

	return targetTransport, nil
}

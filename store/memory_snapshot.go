package store

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/Mathew-Estafanous/raft"
)

// MemorySnapshotStore implements the SnapshotStore interface
// and stores snapshots in memory. This is primarily useful for testing.
type MemorySnapshotStore struct {
	snapshots map[string]*memSnapshot
	mu        sync.RWMutex
}

// NewMemorySnapshotStore creates a new memory-based snapshot store
func NewMemorySnapshotStore() *MemorySnapshotStore {
	return &MemorySnapshotStore{
		snapshots: make(map[string]*memSnapshot),
	}
}

// Create starts a new snapshot creation process
func (m *MemorySnapshotStore) Create(index int64, term uint64, size int64) (raft.SnapshotSink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create a unique ID for the snapshot
	id := fmt.Sprintf("%d-%d-%d", term, index, time.Now().UnixNano())

	// Create the snapshot
	snap := &memSnapshot{
		meta: raft.SnapshotMeta{
			ID:           id,
			Index:        index,
			Term:         term,
			Size:         size,
			CreationTime: time.Now(),
		},
		data: bytes.NewBuffer(nil),
	}

	// Create the sink
	sink := &memSnapshotSink{
		store: m,
		snap:  snap,
	}

	return sink, nil
}

// List returns metadata about available snapshots
func (m *MemorySnapshotStore) List() ([]raft.SnapshotMeta, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var snapshots []raft.SnapshotMeta
	for _, snap := range m.snapshots {
		snapshots = append(snapshots, snap.meta)
	}

	// Sort by index (descending)
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Index > snapshots[j].Index
	})

	return snapshots, nil
}

// Open retrieves a specific snapshot by ID
func (m *MemorySnapshotStore) Open(id string) (*raft.SnapshotData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snap, ok := m.snapshots[id]
	if !ok {
		return nil, raft.ErrSnapshotNotFound
	}

	// Create a reader for the snapshot data
	reader := io.NopCloser(bytes.NewReader(snap.data.Bytes()))

	// Return the snapshot
	return &raft.SnapshotData{
		Meta:   snap.meta,
		Reader: reader,
	}, nil
}

// Latest returns the most recent snapshot
func (m *MemorySnapshotStore) Latest() (*raft.SnapshotData, error) {
	snapshots, err := m.List()
	if err != nil {
		return nil, err
	}

	if len(snapshots) == 0 {
		return nil, raft.ErrSnapshotNotFound
	}

	return m.Open(snapshots[0].ID)
}

// Delete removes a snapshot
func (m *MemorySnapshotStore) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.snapshots[id]; !ok {
		return raft.ErrSnapshotNotFound
	}

	delete(m.snapshots, id)
	return nil
}

// memSnapshot represents a snapshot stored in memory
type memSnapshot struct {
	meta raft.SnapshotMeta
	data *bytes.Buffer
}

// memSnapshotSink implements the SnapshotSink interface for memory snapshots
type memSnapshotSink struct {
	store *MemorySnapshotStore
	snap  *memSnapshot
}

// Write appends data to the snapshot
func (s *memSnapshotSink) Write(p []byte) (n int, err error) {
	return s.snap.data.Write(p)
}

// Close finishes writing the snapshot and releases resources
func (s *memSnapshotSink) Close() error {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	// Update the size
	s.snap.meta.Size = int64(s.snap.data.Len())

	// Store the snapshot
	s.store.snapshots[s.snap.meta.ID] = s.snap

	return nil
}

// ID returns the ID of the snapshot being written
func (s *memSnapshotSink) ID() string {
	return s.snap.meta.ID
}

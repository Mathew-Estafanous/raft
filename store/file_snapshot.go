package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Mathew-Estafanous/raft"
)

// FileSnapshotStore implements the SnapshotStore interface
// and stores snapshots as files on disk
type FileSnapshotStore struct {
	path   string
	retain int
	mu     sync.RWMutex
}

// NewFileSnapshotStore creates a new file-based snapshot store
func NewFileSnapshotStore(path string, retain int) (*FileSnapshotStore, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, err
	}

	store := &FileSnapshotStore{
		path:   path,
		retain: retain,
	}

	return store, nil
}

// Create starts a new snapshot creation process
func (f *FileSnapshotStore) Create(index int64, term uint64, size int64) (raft.SnapshotSink, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	name := fmt.Sprintf("%d-%d-%d.snapshot", term, index, time.Now().UnixNano())
	path := filepath.Join(f.path, name)

	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	sink := &FileSnapshotSink{
		store: f,
		file:  file,
		meta: raft.SnapshotMeta{
			ID:           name,
			Index:        index,
			Term:         term,
			Size:         size,
			CreationTime: time.Now(),
		},
	}

	return sink, nil
}

// List returns metadata about available snapshots
func (f *FileSnapshotStore) List() ([]raft.SnapshotMeta, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	files, err := os.ReadDir(f.path)
	if err != nil {
		return nil, err
	}

	var snapshots []raft.SnapshotMeta
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".snapshot") {
			continue
		}

		meta, err := f.parseFilename(file.Name())
		if err != nil {
			continue
		}

		snapshots = append(snapshots, meta)
	}

	// Sort by index (descending)
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Index > snapshots[j].Index
	})

	return snapshots, nil
}

// Open retrieves a specific snapshot by ID
func (f *FileSnapshotStore) Open(id string) (*raft.SnapshotData, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	path := filepath.Join(f.path, id)
	if _, err := os.Stat(path); err != nil {
		return nil, raft.ErrSnapshotNotFound
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	meta, err := f.parseFilename(id)
	if err != nil {
		file.Close()
		return nil, err
	}

	return &raft.SnapshotData{
		Meta:   meta,
		Reader: file,
	}, nil
}

// Latest returns the most recent snapshot
func (f *FileSnapshotStore) Latest() (*raft.SnapshotData, error) {
	snapshots, err := f.List()
	if err != nil {
		return nil, err
	}

	if len(snapshots) == 0 {
		return nil, raft.ErrSnapshotNotFound
	}

	return f.Open(snapshots[0].ID)
}

// Delete removes a snapshot
func (f *FileSnapshotStore) Delete(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	path := filepath.Join(f.path, id)
	return os.Remove(path)
}

// parseFilename extracts metadata from a snapshot filename
func (f *FileSnapshotStore) parseFilename(filename string) (raft.SnapshotMeta, error) {
	parts := strings.Split(strings.TrimSuffix(filename, ".snapshot"), "-")
	if len(parts) != 3 {
		return raft.SnapshotMeta{}, fmt.Errorf("invalid snapshot filename: %s", filename)
	}

	term, err := parseUint64(parts[0])
	if err != nil {
		return raft.SnapshotMeta{}, err
	}

	index, err := parseInt64(parts[1])
	if err != nil {
		return raft.SnapshotMeta{}, err
	}

	timestamp, err := parseInt64(parts[2])
	if err != nil {
		return raft.SnapshotMeta{}, err
	}

	return raft.SnapshotMeta{
		ID:           filename,
		Index:        index,
		Term:         term,
		Size:         0,
		CreationTime: time.Unix(0, timestamp),
	}, nil
}

// FileSnapshotSink implements the SnapshotSink interface
type FileSnapshotSink struct {
	store  *FileSnapshotStore
	file   *os.File
	meta   raft.SnapshotMeta
	closed bool
}

// Write appends data to the snapshot
func (f *FileSnapshotSink) Write(p []byte) (n int, err error) {
	if f.closed {
		return 0, fmt.Errorf("sink is closed")
	}
	return f.file.Write(p)
}

// Close finishes writing the snapshot and releases resources
func (f *FileSnapshotSink) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	if err := f.file.Close(); err != nil {
		return err
	}

	// Enforce retention policy
	if f.store.retain > 0 {
		snapshots, err := f.store.List()
		if err != nil {
			return err
		}

		// Remove older snapshots if we have too many
		for i := f.store.retain; i < len(snapshots); i++ {
			if err := f.store.Delete(snapshots[i].ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// ID returns the ID of the snapshot being written
func (f *FileSnapshotSink) ID() string {
	return f.meta.ID
}

// Helper functions for parsing
func parseUint64(s string) (uint64, error) {
	var val uint64
	if _, err := fmt.Sscanf(s, "%d", &val); err != nil {
		return 0, err
	}
	return val, nil
}

func parseInt64(s string) (int64, error) {
	var val int64
	if _, err := fmt.Sscanf(s, "%d", &val); err != nil {
		return 0, err
	}
	return val, nil
}

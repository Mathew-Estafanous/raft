package raft

import (
	"errors"
	"io"
	"time"
)

var (
	ErrSnapshotNotFound = errors.New("snapshot could not be found in the storage")
	ErrSnapshotCreation = errors.New("failed to create snapshot")
)

// SnapshotMeta contains metadata about a snapshot
type SnapshotMeta struct {
	// ID is a unique identifier for the snapshot
	ID string

	// Index is the last log index included in the snapshot
	Index int64

	// Term is the term of the last log included in the snapshot
	Term uint64

	// Size is the size of the snapshot in bytes
	Size int64

	// CreationTime is when the snapshot was created
	CreationTime time.Time
}

// SnapshotData represents a point-in-time capture of the FSM state
type SnapshotData struct {
	// Meta contains metadata about the snapshot
	Meta SnapshotMeta

	// Reader provides access to the snapshot data
	Reader io.ReadCloser
}

// SnapshotSink is used to write data to a snapshot
type SnapshotSink interface {
	// Write appends data to the snapshot
	Write(p []byte) (n int, err error)

	// Close finishes writing the snapshot and releases resources
	Close() error

	// ID returns the ID of the snapshot being written
	ID() string
}

// SnapshotStore defines how snapshots are stored and retrieved
type SnapshotStore interface {
	// Create starts a new snapshot creation process and returns a sink
	// that can be used to write the snapshot data
	Create(index int64, term uint64, size int64) (SnapshotSink, error)

	// Open retrieves a specific snapshot by ID
	Open(id string) (*SnapshotData, error)

	// List returns metadata about available snapshots
	List() ([]SnapshotMeta, error)

	// Latest returns the most recent snapshot
	Latest() (*SnapshotData, error)

	// Delete removes a snapshot
	Delete(id string) error
}

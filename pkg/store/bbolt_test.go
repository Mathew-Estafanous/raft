package store

import (
	raft "github.com/Mathew-Estafanous/raft/pkg"
	bolt "go.etcd.io/bbolt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupBoltStore creates a temporary BoltStore for testing
func setupBoltStore(t *testing.T) (*BoltStore, string) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "boltstore-test-*")
	require.NoError(t, err, "Failed to create temp directory")

	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewBoltStore(dbPath)
	require.NoError(t, err, "Failed to create BoltStore")

	return store, tempDir
}

// cleanupBoltStore closes the BoltStore and removes the temporary directory
func cleanupBoltStore(t *testing.T, store *BoltStore, tempDir string) {
	t.Helper()
	err := store.Close()
	require.NoError(t, err, "Failed to close BoltStore")

	err = os.RemoveAll(tempDir)
	require.NoError(t, err, "Failed to remove temp directory")
}

func TestNewBoltStore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "boltstore-test-*")
	require.NoError(t, err, "Failed to create temp directory")
	defer func() {
		err = os.RemoveAll(tempDir)
		assert.NoError(t, err)
	}()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "Valid path",
			path:    filepath.Join(tempDir, "valid.db"),
			wantErr: false,
		},
		{
			name:    "Invalid path",
			path:    filepath.Join("/nonexistent", "invalid.db"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := NewBoltStore(tt.path)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, store)
			assert.Equal(t, int64(-1), store.lastIndex)
			assert.Equal(t, uint64(0), store.lastTerm)

			if store != nil {
				err = store.Close()
				assert.NoError(t, err)
			}
		})
	}
}

func TestBoltStore_Close(t *testing.T) {
	store, tempDir := setupBoltStore(t)
	defer func() {
		err := os.RemoveAll(tempDir)
		assert.NoError(t, err)
	}()

	err := store.Close()
	assert.NoError(t, err)

	_, err = store.Get([]byte("key"))
	assert.Error(t, err)
}

func TestBoltStore_Delete(t *testing.T) {
	store, tempDir := setupBoltStore(t)
	defer cleanupBoltStore(t, store, tempDir)

	err := store.Set([]byte("key"), []byte("value"))
	require.NoError(t, err)

	err = store.Delete()
	assert.NoError(t, err)

	err = store.db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(logBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(metaBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(kvBucket); err != nil {
			return err
		}
		return nil
	})
	require.NoError(t, err)

	value, err := store.Get([]byte("key"))
	assert.NoError(t, err)
	assert.Nil(t, value)
}

func TestBoltStore_LastIndex_LastTerm(t *testing.T) {
	store, tempDir := setupBoltStore(t)
	defer cleanupBoltStore(t, store, tempDir)

	// Initially, lastIndex should be -1 and lastTerm should be 0
	assert.Equal(t, int64(-1), store.LastIndex())
	assert.Equal(t, uint64(0), store.LastTerm())

	logs := []*raft.Log{
		{
			Type:  raft.Entry,
			Index: 1,
			Term:  1,
			Cmd:   []byte("command1"),
		},
		{
			Type:  raft.Entry,
			Index: 2,
			Term:  1,
			Cmd:   []byte("command2"),
		},
		{
			Type:  raft.Entry,
			Index: 3,
			Term:  2,
			Cmd:   []byte("command3"),
		},
	}
	err := store.AppendLogs(logs)
	require.NoError(t, err)

	assert.Equal(t, int64(3), store.LastIndex())
	assert.Equal(t, uint64(2), store.LastTerm())
}

func TestBoltStore_GetLog(t *testing.T) {
	store, tempDir := setupBoltStore(t)
	defer cleanupBoltStore(t, store, tempDir)

	logs := []*raft.Log{
		{
			Type:  raft.Entry,
			Index: 1,
			Term:  1,
			Cmd:   []byte("command1"),
		},
		{
			Type:  raft.Entry,
			Index: 2,
			Term:  1,
			Cmd:   []byte("command2"),
		},
		{
			Type:  raft.Entry,
			Index: 3,
			Term:  2,
			Cmd:   []byte("command3"),
		},
	}
	err := store.AppendLogs(logs)
	require.NoError(t, err)

	tests := []struct {
		name      string
		index     int64
		wantLog   *raft.Log
		wantError error
	}{
		{
			name:      "Get existing log",
			index:     2,
			wantLog:   logs[1],
			wantError: nil,
		},
		{
			name:      "Get first log",
			index:     1,
			wantLog:   logs[0],
			wantError: nil,
		},
		{
			name:      "Get last log",
			index:     3,
			wantLog:   logs[2],
			wantError: nil,
		},
		{
			name:      "Index too high",
			index:     4,
			wantLog:   nil,
			wantError: raft.ErrLogNotFound,
		},
		{
			name:      "Index too low",
			index:     0,
			wantLog:   logs[0],
			wantError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, err := store.GetLog(tt.index)
			if tt.wantError != nil {
				assert.Equal(t, tt.wantError, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantLog.Type, log.Type)
			assert.Equal(t, tt.wantLog.Index, log.Index)
			assert.Equal(t, tt.wantLog.Term, log.Term)
			assert.Equal(t, tt.wantLog.Cmd, log.Cmd)
		})
	}
}

func TestBoltStore_AllLogs(t *testing.T) {
	store, tempDir := setupBoltStore(t)
	defer cleanupBoltStore(t, store, tempDir)

	// Test with empty store
	logs, err := store.AllLogs()
	assert.NoError(t, err)
	assert.Empty(t, logs)

	// Add some logs
	inputLogs := []*raft.Log{
		{
			Type:  raft.Entry,
			Index: 1,
			Term:  1,
			Cmd:   []byte("command1"),
		},
		{
			Type:  raft.Entry,
			Index: 2,
			Term:  1,
			Cmd:   []byte("command2"),
		},
		{
			Type:  raft.Entry,
			Index: 3,
			Term:  2,
			Cmd:   []byte("command3"),
		},
	}
	err = store.AppendLogs(inputLogs)
	require.NoError(t, err)

	logs, err = store.AllLogs()
	assert.NoError(t, err)
	assert.Len(t, logs, 3)

	for i, log := range logs {
		assert.Equal(t, inputLogs[i].Type, log.Type)
		assert.Equal(t, inputLogs[i].Index, log.Index)
		assert.Equal(t, inputLogs[i].Term, log.Term)
		assert.Equal(t, inputLogs[i].Cmd, log.Cmd)
	}
}

func TestBoltStore_AppendLogs(t *testing.T) {
	store, tempDir := setupBoltStore(t)
	defer cleanupBoltStore(t, store, tempDir)

	// Test with empty logs slice
	err := store.AppendLogs([]*raft.Log{})
	assert.NoError(t, err)

	logs1 := []*raft.Log{
		{
			Type:  raft.Entry,
			Index: 1,
			Term:  1,
			Cmd:   []byte("command1"),
		},
		{
			Type:  raft.Entry,
			Index: 2,
			Term:  1,
			Cmd:   []byte("command2"),
		},
	}
	err = store.AppendLogs(logs1)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), store.LastIndex())
	assert.Equal(t, uint64(1), store.LastTerm())

	logs2 := []*raft.Log{
		{
			Type:  raft.Entry,
			Index: 3,
			Term:  2,
			Cmd:   []byte("command3"),
		},
		{
			Type:  raft.Entry,
			Index: 4,
			Term:  2,
			Cmd:   []byte("command4"),
		},
	}
	err = store.AppendLogs(logs2)
	assert.NoError(t, err)
	assert.Equal(t, int64(4), store.LastIndex())
	assert.Equal(t, uint64(2), store.LastTerm())

	allLogs, err := store.AllLogs()
	assert.NoError(t, err)
	assert.Len(t, allLogs, 4)
}

func TestBoltStore_DeleteRange(t *testing.T) {
	logs := []*raft.Log{
		{
			Type:  raft.Entry,
			Index: 1,
			Term:  1,
			Cmd:   []byte("command1"),
		},
		{
			Type:  raft.Entry,
			Index: 2,
			Term:  1,
			Cmd:   []byte("command2"),
		},
		{
			Type:  raft.Entry,
			Index: 3,
			Term:  2,
			Cmd:   []byte("command3"),
		},
		{
			Type:  raft.Entry,
			Index: 4,
			Term:  2,
			Cmd:   []byte("command4"),
		},
		{
			Type:  raft.Entry,
			Index: 5,
			Term:  3,
			Cmd:   []byte("command5"),
		},
	}

	tests := []struct {
		name          string
		min           int64
		max           int64
		wantErr       bool
		wantLastIndex int64
		wantLastTerm  uint64
		wantLogCount  int
	}{
		{
			name:          "Delete middle range",
			min:           2,
			max:           3,
			wantErr:       false,
			wantLastIndex: 5,
			wantLastTerm:  3,
			wantLogCount:  3,
		},
		{
			name:          "Delete to the end",
			min:           4,
			max:           5,
			wantErr:       false,
			wantLastIndex: 3,
			wantLastTerm:  2,
			wantLogCount:  3,
		},
		{
			name:          "Delete all logs",
			min:           1,
			max:           5,
			wantErr:       false,
			wantLastIndex: -1,
			wantLastTerm:  0,
			wantLogCount:  0,
		},
		{
			name:          "Deletes up to max even if max > lastIndex",
			min:           4,
			max:           6,
			wantErr:       false,
			wantLastIndex: 3,
			wantLastTerm:  2,
			wantLogCount:  3,
		},
		{
			name:          "Invalid range (min < existing min)",
			min:           0,
			max:           1,
			wantErr:       true,
			wantLastIndex: -1,
			wantLastTerm:  0,
			wantLogCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, tempDir := setupBoltStore(t)
			defer cleanupBoltStore(t, store, tempDir)

			err := store.AppendLogs(logs)
			require.NoError(t, err)

			err = store.DeleteRange(tt.min, tt.max)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			// Verify lastIndex and lastTerm
			assert.Equal(t, tt.wantLastIndex, store.LastIndex())
			assert.Equal(t, tt.wantLastTerm, store.LastTerm())

			// Verify log count
			allLogs, err := store.AllLogs()
			assert.NoError(t, err)
			assert.Len(t, allLogs, tt.wantLogCount)
		})
	}
}

func TestBoltStore_Set_Get(t *testing.T) {
	store, tempDir := setupBoltStore(t)
	defer cleanupBoltStore(t, store, tempDir)

	tests := []struct {
		name  string
		key   []byte
		value []byte
	}{
		{
			name:  "Simple key-value",
			key:   []byte("key1"),
			value: []byte("value1"),
		},
		{
			name:  "Empty value",
			key:   []byte("key2"),
			value: []byte{},
		},
		{
			name:  "Binary data",
			key:   []byte{0x00, 0x01, 0x02},
			value: []byte{0x03, 0x04, 0x05},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the key-value pair
			err := store.Set(tt.key, tt.value)
			assert.NoError(t, err)

			// Get the value
			value, err := store.Get(tt.key)
			assert.NoError(t, err)
			assert.Equal(t, tt.value, value)
		})
	}

	t.Run("Non-existent key", func(t *testing.T) {
		value, err := store.Get([]byte("nonexistent"))
		assert.NoError(t, err)
		assert.Nil(t, value)
	})

	t.Run("Overwrite key", func(t *testing.T) {
		key := []byte("overwrite")
		value1 := []byte("value1")
		value2 := []byte("value2")

		err := store.Set(key, value1)
		assert.NoError(t, err)

		err = store.Set(key, value2)
		assert.NoError(t, err)

		value, err := store.Get(key)
		assert.NoError(t, err)
		assert.Equal(t, value2, value)
	})
}

package store

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Mathew-Estafanous/raft"
	bolt "go.etcd.io/bbolt"
)

var (
	logBucket  = []byte("logs")
	metaBucket = []byte("meta")
	kvBucket   = []byte("kv")

	lastIndexKey = []byte("lastIndex")
	lastTermKey  = []byte("lastTerm")
)

// BoltStore implements both raft.LogStore and raft.StableStore interfaces
// using BBolt as the underlying storage engine
type BoltStore struct {
	db        *bolt.DB
	mu        sync.RWMutex
	lastIndex int64
	lastTerm  uint64
}

// NewBoltStore creates a new store persisted at the given path
func NewBoltStore(path string) (*BoltStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}

	err = db.Update(func(tx *bolt.Tx) error {
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
	if err != nil {
		return nil, fmt.Errorf("failed to create buckets: %w", err)
	}

	store := &BoltStore{
		db:        db,
		lastIndex: -1,
		lastTerm:  0,
	}

	err = db.View(func(tx *bolt.Tx) error {
		metaBkt := tx.Bucket(metaBucket)

		lastIdxBytes := metaBkt.Get(lastIndexKey)
		if lastIdxBytes != nil {
			store.lastIndex = int64(binary.BigEndian.Uint64(lastIdxBytes))
		}

		lastTermBytes := metaBkt.Get(lastTermKey)
		if lastTermBytes != nil {
			store.lastTerm = binary.BigEndian.Uint64(lastTermBytes)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return store, nil
}

// Close closes the underlying BBolt database
func (b *BoltStore) Close() error {
	return b.db.Close()
}

// Delete will completely erase all data in the database
func (b *BoltStore) Delete() error {
	return b.db.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket(logBucket); err != nil {
			return err
		}
		if err := tx.DeleteBucket(metaBucket); err != nil {
			return err
		}
		if err := tx.DeleteBucket(kvBucket); err != nil {
			return err
		}
		return nil
	})
}

// LastIndex returns the last index in the log
func (b *BoltStore) LastIndex() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lastIndex
}

// LastTerm returns the term of the last log entry
func (b *BoltStore) LastTerm() uint64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lastTerm
}

// GetLog retrieves a log entry at a specific index
func (b *BoltStore) GetLog(index int64) (*raft.Log, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var log *raft.Log
	err := b.db.View(func(tx *bolt.Tx) error {
		logBkt := tx.Bucket(logBucket)

		if b.lastIndex < index {
			return raft.ErrLogNotFound
		}

		c := logBkt.Cursor()
		k, _ := c.First()
		if k == nil {
			return raft.ErrLogNotFound
		}

		minIdx := int64(binary.BigEndian.Uint64(k))

		if index < minIdx {
			data := logBkt.Get(k)
			if data == nil {
				return raft.ErrLogNotFound
			}

			log = &raft.Log{}
			return json.Unmarshal(data, log)
		}

		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))
		data := logBkt.Get(key)
		if data == nil {
			return raft.ErrLogNotFound
		}

		log = &raft.Log{}
		return json.Unmarshal(data, log)
	})

	if err != nil {
		return nil, err
	}
	return log, nil
}

// AllLogs retrieves all log entries
func (b *BoltStore) AllLogs() ([]*raft.Log, error) {
	var logs []*raft.Log
	err := b.db.View(func(tx *bolt.Tx) error {
		logBkt := tx.Bucket(logBucket)

		return logBkt.ForEach(func(k, v []byte) error {
			log := &raft.Log{}
			if err := json.Unmarshal(v, log); err != nil {
				return err
			}
			logs = append(logs, log)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}
	return logs, nil
}

// AppendLogs adds new log entries
func (b *BoltStore) AppendLogs(logs []*raft.Log) error {
	if len(logs) == 0 {
		return nil
	}

	err := b.db.Update(func(tx *bolt.Tx) error {
		logBkt := tx.Bucket(logBucket)
		metaBkt := tx.Bucket(metaBucket)

		for _, log := range logs {
			data, err := json.Marshal(log)
			if err != nil {
				return err
			}

			key := make([]byte, 8)
			binary.BigEndian.PutUint64(key, uint64(log.Index))

			if err := logBkt.Put(key, data); err != nil {
				return err
			}

			if log.Index > b.lastIndex {
				b.mu.Lock()
				b.lastIndex = log.Index
				b.lastTerm = log.Term
				b.mu.Unlock()

				idxKey := make([]byte, 8)
				binary.BigEndian.PutUint64(idxKey, uint64(b.lastIndex))
				if err := metaBkt.Put(lastIndexKey, idxKey); err != nil {
					return err
				}

				termKey := make([]byte, 8)
				binary.BigEndian.PutUint64(termKey, b.lastTerm)
				if err := metaBkt.Put(lastTermKey, termKey); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return err
}

// DeleteRange removes log entries in the specified range
func (b *BoltStore) DeleteRange(min, max int64) error {
	err := b.db.Update(func(tx *bolt.Tx) error {
		logBkt := tx.Bucket(logBucket)

		c := logBkt.Cursor()
		k, _ := c.First()
		if k == nil {
			return fmt.Errorf("no logs found")
		}

		minIdx := int64(binary.BigEndian.Uint64(k))
		if min < minIdx {
			return fmt.Errorf("min %v cannot be less than the current minimum index %v", min, minIdx)
		}

		for i := min; i <= max; i++ {
			key := make([]byte, 8)
			binary.BigEndian.PutUint64(key, uint64(i))
			if err := logBkt.Delete(key); err != nil {
				return err
			}
		}

		if max >= b.lastIndex {
			k, v := c.Last()
			b.mu.Lock()
			if k == nil {
				b.lastIndex = -1
				b.lastTerm = 0
			} else {
				b.lastIndex = int64(binary.BigEndian.Uint64(k))
				log := &raft.Log{}
				if err := json.Unmarshal(v, log); err != nil {
					b.mu.Unlock()
					return err
				}
				b.lastTerm = log.Term
			}
			b.mu.Unlock()
			metaBkt := tx.Bucket(metaBucket)

			idxKey := make([]byte, 8)
			binary.BigEndian.PutUint64(idxKey, uint64(b.lastIndex))
			if err := metaBkt.Put(lastIndexKey, idxKey); err != nil {
				return err
			}

			termKey := make([]byte, 8)
			binary.BigEndian.PutUint64(termKey, b.lastTerm)
			if err := metaBkt.Put(lastTermKey, termKey); err != nil {
				return err
			}
		}
		return nil
	})

	return err
}

// Set stores a key-value pair in the stable store
func (b *BoltStore) Set(key, value []byte) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(kvBucket).Put(key, value)
	})
}

// Get retrieves a value by key from the stable store
func (b *BoltStore) Get(key []byte) ([]byte, error) {
	var value []byte
	err := b.db.View(func(tx *bolt.Tx) error {
		val := tx.Bucket(kvBucket).Get(key)
		if val != nil {
			value = append([]byte{}, val...)
		}
		return nil
	})
	return value, err
}

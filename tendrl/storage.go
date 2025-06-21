package tendrl

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Storage handles persistent message storage using BoltDB
type Storage struct {
	db *bolt.DB
}

// StoredMessage represents a message in storage with TTL
type StoredMessage struct {
	Data      string   `json:"data"`
	Tags      []string `json:"tags,omitempty"`
	Timestamp int64    `json:"timestamp"`
	TTL       int64    `json:"ttl"`
}

// NewStorage creates a new BoltDB storage instance
func NewStorage(path string) (*Storage, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("could not open db: %v", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("messages"))
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("could not create bucket: %v", err)
	}

	return &Storage{db: db}, nil
}

// Store saves a message with TTL
func (s *Storage) Store(id string, data string, tags []string, ttl int64) error {
	msg := StoredMessage{
		Data:      data,
		Tags:      tags,
		Timestamp: time.Now().Unix(),
		TTL:       ttl,
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("messages"))
		encoded, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), encoded)
	})
}

// Get retrieves a message by ID
func (s *Storage) Get(id string) (*StoredMessage, error) {
	var msg StoredMessage

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("messages"))
		v := b.Get([]byte(id))
		if v == nil {
			return fmt.Errorf("message not found")
		}
		return json.Unmarshal(v, &msg)
	})

	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// CleanupExpired removes messages that have exceeded their TTL
func (s *Storage) CleanupExpired() error {
	now := time.Now().Unix()

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("messages"))
		return b.ForEach(func(k, v []byte) error {
			var msg StoredMessage
			if err := json.Unmarshal(v, &msg); err != nil {
				return err
			}
			if now >= msg.Timestamp+msg.TTL {
				if err := b.Delete(k); err != nil {
					return err
				}
			}
			return nil
		})
	})
}

// Close closes the database connection
func (s *Storage) Close() error {
	return s.db.Close()
}

// GetBatch retrieves a limited batch of stored messages for retry processing
func (s *Storage) GetBatch(limit int) (map[string]*StoredMessage, error) {
	messages := make(map[string]*StoredMessage)
	count := 0

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("messages"))
		return b.ForEach(func(k, v []byte) error {
			if count >= limit {
				return nil // Stop iteration when limit reached
			}

			var msg StoredMessage
			if err := json.Unmarshal(v, &msg); err != nil {
				return err
			}
			messages[string(k)] = &msg
			count++
			return nil
		})
	})

	return messages, err
}

// DeleteBatch removes multiple messages by IDs in a single transaction
func (s *Storage) DeleteBatch(ids []string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("messages"))
		for _, id := range ids {
			if err := b.Delete([]byte(id)); err != nil {
				return err
			}
		}
		return nil
	})
}

// Delete removes a message by ID
func (s *Storage) Delete(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("messages"))
		return b.Delete([]byte(id))
	})
}

// Count returns the number of stored messages
func (s *Storage) Count() (int, error) {
	var count int

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("messages"))
		stats := b.Stats()
		count = stats.KeyN
		return nil
	})

	return count, err
}

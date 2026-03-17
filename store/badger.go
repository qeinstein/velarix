package store

import (
	"encoding/json"
	"fmt"
	"time"

	"velarix/core"
	"github.com/dgraph-io/badger/v4"
)

type BadgerStore struct {
	db *badger.DB
}

func OpenBadger(path string, encryptionKey []byte) (*BadgerStore, error) {
	opts := badger.DefaultOptions(path).
		WithLogger(nil).
		WithNumVersionsToKeep(1).
		WithValueThreshold(1024) // 1KB threshold for value log

	if len(encryptionKey) > 0 {
		// Badger requires 16, 24, or 32 bytes for AES
		if len(encryptionKey) != 16 && len(encryptionKey) != 24 && len(encryptionKey) != 32 {
			return nil, fmt.Errorf("invalid encryption key length: %d bytes (must be 16, 24, or 32)", len(encryptionKey))
		}
		opts = opts.WithEncryptionKey(encryptionKey)
		opts = opts.WithIndexCacheSize(100 << 20) // Recommended when encryption is on
	}

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return &BadgerStore{db: db}, nil
}

func (s *BadgerStore) Close() error {
	return s.db.Close()
}

// StartGC runs the value log garbage collector in a background goroutine.
func (s *BadgerStore) StartGC() {
	ticker := time.NewTicker(30 * time.Minute)
	go func() {
		for range ticker.C {
			for {
				err := s.db.RunValueLogGC(0.5)
				if err != nil {
					break
				}
			}
		}
	}()
}

// Append persists an entry and tags it by session
func (s *BadgerStore) Append(entry JournalEntry) error {
	if entry.Timestamp == 0 {
		entry.Timestamp = time.Now().UnixMilli()
	}
	
	val, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	// Key: s:{session_id}:h:{timestamp}
	// This allows O(K) range scans per session
	key := []byte(fmt.Sprintf("s:%s:h:%d", entry.SessionID, entry.Timestamp))

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, val)
	})
}

// GetSessionHistory returns history for ONE session only (O(K) lookup)
func (s *BadgerStore) GetSessionHistory(sessionID string) ([]JournalEntry, error) {
	history := make([]JournalEntry, 0)
	prefix := []byte(fmt.Sprintf("s:%s:h:", sessionID))

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			var entry JournalEntry
			item := it.Item()
			err := item.Value(func(v []byte) error {
				return json.Unmarshal(v, &entry)
			})
			if err != nil {
				return err
			}
			history = append(history, entry)
		}
		return nil
	})

	return history, err
}

// SaveConfig persists session-specific settings
func (s *BadgerStore) SaveConfig(sessionID string, config interface{}) error {
	val, err := json.Marshal(config)
	if err != nil {
		return err
	}
	key := []byte(fmt.Sprintf("s:%s:c", sessionID))
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, val)
	})
}

type SessionConfig struct {
	Schema          string `json:"schema"`
	EnforcementMode string `json:"enforcement_mode"` // "strict" or "warn"
}

type APIKey struct {
	Key        string `json:"key"`
	Label      string `json:"label"`
	CreatedAt  int64  `json:"created_at"`
	LastUsedAt int64  `json:"last_used_at"`
	IsRevoked  bool   `json:"is_revoked"`
}

type User struct {
	Email          string   `json:"email"`
	HashedPassword string   `json:"hashed_password"`
	OrgID          string   `json:"org_id"`
	Keys           []APIKey `json:"keys"`
	ResetToken     string   `json:"reset_token,omitempty"`
	ResetExpiry    int64    `json:"reset_expiry,omitempty"`
}

// GetUser retrieves a user by email
func (s *BadgerStore) GetUser(email string) (*User, error) {
	key := []byte("u:" + email)
	var user User
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error {
			return json.Unmarshal(v, &user)
		})
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// SaveUser persists or updates a user
func (s *BadgerStore) SaveUser(user *User) error {
	val, err := json.Marshal(user)
	if err != nil {
		return err
	}
	
	return s.db.Update(func(txn *badger.Txn) error {
		// Save user record
		if err := txn.Set([]byte("u:"+user.Email), val); err != nil {
			return err
		}
		
		// Map every active key to this user for fast lookup in middleware
		for _, k := range user.Keys {
			if !k.IsRevoked {
				if err := txn.Set([]byte("k:"+k.Key), []byte(user.Email)); err != nil {
					return err
				}
			} else {
				// Ensure revoked keys are removed from the lookup map
				txn.Delete([]byte("k:" + k.Key))
			}
		}
		return nil
	})
}

// GetConfig retrieves session configuration
func (s *BadgerStore) GetConfig(sessionID string) (*SessionConfig, error) {
	var config SessionConfig
	key := []byte(fmt.Sprintf("s:%s:c", sessionID))
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error {
			return json.Unmarshal(v, &config)
		})
	})
	return &config, err
}

// SaveSnapshot persists a binary state snapshot
func (s *BadgerStore) SaveSnapshot(sessionID string, snap *core.Snapshot) error {
	val, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	key := []byte(fmt.Sprintf("s:%s:snap", sessionID))
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, val)
	})
}

// GetLatestSnapshot retrieves the most recent snapshot for a session
func (s *BadgerStore) GetLatestSnapshot(sessionID string) (*core.Snapshot, error) {
	key := []byte(fmt.Sprintf("s:%s:snap", sessionID))
	var snap core.Snapshot
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error {
			return json.Unmarshal(v, &snap)
		})
	})
	if err != nil {
		return nil, err
	}
	return &snap, nil
}

// GetAPIKeyOwner returns the email of the user who owns the given key
func (s *BadgerStore) GetAPIKeyOwner(key string) ([]byte, error) {
	var email []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("k:" + key))
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error {
			email = make([]byte, len(v))
			copy(email, v)
			return nil
		})
	})
	return email, err
}

// SaveAPIKey stores a new API key
func (s *BadgerStore) SaveAPIKey(key, email string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte("k:"+key), []byte(email))
	})
}

// ValidateAPIKey checks if a key exists
func (s *BadgerStore) ValidateAPIKey(key string) (bool, error) {
	exists := false
	err := s.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte("k:" + key))
		if err == badger.ErrKeyNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		exists = true
		return nil
	})
	return exists, err
}

// ReplayAll loads all data into memory on startup
func (s *BadgerStore) ReplayAll(engines map[string]*core.Engine, configs map[string][]byte) error {
	return s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := string(item.Key())

			// Handle Configs: s:{id}:c
			if len(key) > 4 && key[0:2] == "s:" && key[len(key)-2:] == ":c" {
				sessionID := key[2 : len(key)-2]
				err := item.Value(func(v []byte) error {
					b := make([]byte, len(v))
					copy(b, v)
					configs[sessionID] = b
					return nil
				})
				if err != nil {
					return err
				}
				continue
			}

			// Handle History/Replay: s:{id}:h:{ts}
			// We check if it matches the history prefix pattern
			if len(key) > 6 && key[0:2] == "s:" && containsHistoryTag(key) {
				var entry JournalEntry
				err := item.Value(func(v []byte) error {
					return json.Unmarshal(v, &entry)
				})
				if err != nil {
					return err
				}

				engine, ok := engines[entry.SessionID]
				if !ok {
					engine = core.NewEngine()
					engines[entry.SessionID] = engine
				}

				if entry.Type == EventAssert {
					engine.AssertFact(entry.Fact)
				} else if entry.Type == EventInvalidate {
					engine.InvalidateRoot(entry.FactID)
				}
			}
		}
		return nil
	})
}

func containsHistoryTag(key string) bool {
	// Simple check for :h: in the key
	for i := 0; i < len(key)-3; i++ {
		if key[i:i+3] == ":h:" {
			return true
		}
	}
	return false
}

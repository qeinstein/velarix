package store

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
	"velarix/core"
)

// Merge operator for atomic uint64 addition
func uint64Add(originalValue, newValue []byte) []byte {
	var existing uint64
	if len(originalValue) > 0 {
		existing = binary.BigEndian.Uint64(originalValue)
	}
	added := binary.BigEndian.Uint64(newValue)

	res := make([]byte, 8)
	binary.BigEndian.PutUint64(res, existing+added)
	return res
}

type BadgerStore struct {
	db *badger.DB
}

const DBVersion = 1

func OpenBadger(path string, encryptionKey []byte) (*BadgerStore, error) {
	opts := badger.DefaultOptions(path).
		WithLogger(nil).
		WithNumVersionsToKeep(1).
		WithSyncWrites(true).    // Critical: Ensure durable writes
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
	s := &BadgerStore{db: db}
	if err := s.ensureMigrations(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *BadgerStore) ensureMigrations() error {
	var currentVersion uint64
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("sys:version"))
		if err == badger.ErrKeyNotFound {
			currentVersion = 0
			return nil
		}
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error {
			currentVersion = binary.BigEndian.Uint64(v)
			return nil
		})
	})
	if err != nil {
		return err
	}

	if currentVersion < DBVersion {
		// Perform migrations sequentially
		for v := currentVersion + 1; v <= DBVersion; v++ {
			if err := s.migrate(v); err != nil {
				return fmt.Errorf("migration to version %d failed: %v", v, err)
			}
		}

		// Update version key
		return s.db.Update(func(txn *badger.Txn) error {
			verBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(verBytes, uint64(DBVersion))
			return txn.Set([]byte("sys:version"), verBytes)
		})
	}
	return nil
}

type MigrationFunc func(txn *badger.Txn) error

var migrations = map[uint64]MigrationFunc{
	1: func(txn *badger.Txn) error {
		// Migration to v1: Initial schema
		// (Already assumed in initial DB state, so this can be a no-op or sanity check)
		return nil
	},
}

func (s *BadgerStore) migrate(version uint64) error {
	mFunc, ok := migrations[version]
	if !ok {
		return fmt.Errorf("no migration found for version %d", version)
	}

	slog.Info("Running migration", "version", version)
	return s.db.Update(func(txn *badger.Txn) error {
		return mFunc(txn)
	})
}

func (s *BadgerStore) DB() *badger.DB {
	return s.db
}

// IncrementMetric atomically increments a 64-bit counter for an organization
func (s *BadgerStore) IncrementMetric(orgID string, metric string) error {
	key := []byte(fmt.Sprintf("org:%s:m:%s", orgID, metric))
	return s.db.Update(func(txn *badger.Txn) error {
		current := uint64(0)
		if item, err := txn.Get(key); err == nil {
			err = item.Value(func(v []byte) error {
				if len(v) == 8 {
					current = binary.BigEndian.Uint64(v)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, current+1)
		return txn.Set(key, buf)
	})
}

// GetOrgUsage retrieves all 6 metrics for an organization
func (s *BadgerStore) GetOrgUsage(orgID string) (map[string]uint64, error) {
	metrics := []string{
		"api_requests",
		"facts_asserted",
		"schema_violations",
		"facts_pruned",
		"sessions_created",
		"revalidation_runs",
	}

	result := make(map[string]uint64)
	err := s.db.View(func(txn *badger.Txn) error {
		for _, m := range metrics {
			key := []byte(fmt.Sprintf("org:%s:m:%s", orgID, m))
			item, err := txn.Get(key)
			if err == badger.ErrKeyNotFound {
				result[m] = 0
				continue
			}
			if err != nil {
				return err
			}
			err = item.Value(func(v []byte) error {
				if len(v) == 8 {
					result[m] = binary.BigEndian.Uint64(v)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return result, err
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
	now := time.Now()
	if entry.Timestamp == 0 {
		entry.Timestamp = now.UnixMilli()
	}

	val, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	// Key: s:{session_id}:h:{timestamp_nano}
	// This allows O(K) range scans per session and avoids collisions
	key := []byte(fmt.Sprintf("s:%s:h:%020d", entry.SessionID, now.UnixNano()))

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
	ExpiresAt  int64  `json:"expires_at"` // Add expiration for key rotation
	IsRevoked  bool   `json:"is_revoked"`
}

type Organization struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	CreatedAt   int64  `json:"created_at"`
	IsSuspended bool   `json:"is_suspended"`
}

type User struct {
	Email          string   `json:"email"`
	HashedPassword string   `json:"hashed_password"`
	OrgID          string   `json:"org_id"`
	Role           string   `json:"role"` // "admin" or "member"
	Keys           []APIKey `json:"keys"`
	ResetToken     string   `json:"reset_token,omitempty"`
	ResetExpiry    int64    `json:"reset_expiry,omitempty"`
}

// GetOrganization retrieves an organization by ID
func (s *BadgerStore) GetOrganization(id string) (*Organization, error) {
	key := []byte("o:" + id)
	var org Organization
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error {
			return json.Unmarshal(v, &org)
		})
	})
	if err != nil {
		return nil, err
	}
	return &org, nil
}

// SaveOrganization persists an organization
func (s *BadgerStore) SaveOrganization(org *Organization) error {
	val, err := json.Marshal(org)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte("o:"+org.ID), val)
	})
}

// SetSessionOrganization links a session to an organization
func (s *BadgerStore) SetSessionOrganization(sessionID, orgID string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte("s:"+sessionID+":org"), []byte(orgID))
	})
}

// GetSessionOrganization retrieves the organization ID for a session
func (s *BadgerStore) GetSessionOrganization(sessionID string) (string, error) {
	var orgID string
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("s:" + sessionID + ":org"))
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error {
			orgID = string(v)
			return nil
		})
	})
	return orgID, err
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

// GetRateLimit retrieves the list of timestamps for an API key's rate limit window
func (s *BadgerStore) GetRateLimit(apiKey string) ([]time.Time, error) {
	key := []byte("rl:" + apiKey)
	var limits []time.Time
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error {
			return json.Unmarshal(v, &limits)
		})
	})
	return limits, err
}

// SaveRateLimit persists the rate limit window timestamps for an API key
func (s *BadgerStore) SaveRateLimit(apiKey string, limits []time.Time) error {
	key := []byte("rl:" + apiKey)
	val, err := json.Marshal(limits)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, val)
	})
}

// Backup creates a full database backup
func (s *BadgerStore) Backup(w io.Writer) (uint64, error) {
	return s.db.Backup(w, 0)
}

// Restore loads a full database backup
func (s *BadgerStore) Restore(r io.Reader) error {
	return s.db.Load(r, 16)
}

// ReplayAll loads all data into memory on startup efficiently using snapshots
func (s *BadgerStore) ReplayAll(engines map[string]*core.Engine, configs map[string][]byte) error {
	snapshotTimestamps := make(map[string]int64)

	return s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		// Pass 1: Load Snapshots and Configs
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

			// Handle Snapshots: s:{id}:snap
			if len(key) > 7 && key[0:2] == "s:" && strings.HasSuffix(key, ":snap") {
				sessionID := key[2 : len(key)-5]
				var snap core.Snapshot
				err := item.Value(func(v []byte) error {
					return json.Unmarshal(v, &snap)
				})
				if err != nil {
					slog.Warn("Failed to decode snapshot during replay", "session_id", sessionID, "error", err)
					continue
				}

				engine := core.NewEngine()
				if err := engine.FromSnapshot(&snap); err == nil {
					engines[sessionID] = engine
					snapshotTimestamps[sessionID] = snap.Timestamp
				} else {
					slog.Warn("Failed to load snapshot during replay", "session_id", sessionID, "error", err)
				}
			}
		}

		// Pass 2: Replay History for entries AFTER snapshots
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := string(item.Key())

			if len(key) > 6 && key[0:2] == "s:" && containsHistoryTag(key) {
				var entry JournalEntry
				err := item.Value(func(v []byte) error {
					return json.Unmarshal(v, &entry)
				})
				if err != nil {
					slog.Warn("Skipping corrupt journal entry", "key", key, "error", err)
					continue
				}

				// Skip if we have a newer snapshot
				if ts, ok := snapshotTimestamps[entry.SessionID]; ok && entry.Timestamp <= ts {
					continue
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

// --- Explanation Persistence ---

// ExplanationRecord stores an immutable explanation with integrity hash.
type ExplanationRecord struct {
	SessionID   string          `json:"session_id"`
	Timestamp   int64           `json:"timestamp"`
	Content     json.RawMessage `json:"content"`
	ContentHash string          `json:"content_hash"`
	Tampered    bool            `json:"tampered"`
}

// SaveExplanation persists an explanation with a SHA-256 integrity hash.
func (s *BadgerStore) SaveExplanation(sessionID string, content json.RawMessage) (*ExplanationRecord, error) {
	now := time.Now()
	hash := sha256Hash(content)

	record := ExplanationRecord{
		SessionID:   sessionID,
		Timestamp:   now.UnixMilli(),
		Content:     content,
		ContentHash: hash,
		Tampered:    false,
	}

	val, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}

	key := []byte(fmt.Sprintf("explanations:%s:%020d", sessionID, now.UnixNano()))
	err = s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, val)
	})
	if err != nil {
		return nil, err
	}

	return &record, nil
}

// GetSessionExplanations returns all stored explanations with tamper verification.
func (s *BadgerStore) GetSessionExplanations(sessionID string) ([]ExplanationRecord, error) {
	var records []ExplanationRecord
	prefix := []byte(fmt.Sprintf("explanations:%s:", sessionID))

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			var record ExplanationRecord
			err := it.Item().Value(func(v []byte) error {
				return json.Unmarshal(v, &record)
			})
			if err != nil {
				continue
			}

			// Verify integrity
			expectedHash := sha256Hash(record.Content)
			if expectedHash != record.ContentHash {
				record.Tampered = true
			}

			records = append(records, record)
		}
		return nil
	})

	return records, err
}

func (s *BadgerStore) GetSessionHistoryBefore(sessionID string, beforeTimestamp int64) ([]JournalEntry, error) {
	history := make([]JournalEntry, 0)
	prefix := []byte(fmt.Sprintf("s:%s:h:", sessionID))

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			var entry JournalEntry
			err := it.Item().Value(func(v []byte) error {
				return json.Unmarshal(v, &entry)
			})
			if err != nil {
				continue
			}
			if entry.Timestamp <= beforeTimestamp {
				history = append(history, entry)
			}
		}
		return nil
	})

	return history, err
}

func sha256Hash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

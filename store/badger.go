package store

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
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

// BadgerStore is the local Badger-backed adapter used for development, tests, and migration bridges.
type BadgerStore struct {
	db          *badger.DB
	appendLocks sync.Map // session_id -> *sync.Mutex
}

func (s *BadgerStore) sessionVersionKey(sessionID string) []byte {
	return []byte(fmt.Sprintf("s:%s:version", sessionID))
}

func (s *BadgerStore) bumpSessionVersionTxn(txn *badger.Txn, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	key := s.sessionVersionKey(sessionID)
	var version uint64
	if item, err := txn.Get(key); err == nil {
		_ = item.Value(func(v []byte) error {
			if len(v) == 8 {
				version = binary.BigEndian.Uint64(v)
			}
			return nil
		})
	}
	version++
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, version)
	return txn.Set(key, buf)
}

func (s *BadgerStore) GetSessionVersion(sessionID string) (int64, error) {
	var version int64
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(s.sessionVersionKey(sessionID))
		if err == badger.ErrKeyNotFound {
			version = 0
			return nil
		}
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error {
			if len(v) == 8 {
				version = int64(binary.BigEndian.Uint64(v))
			}
			return nil
		})
	})
	return version, err
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

func (s *BadgerStore) appendLock(sessionID string) *sync.Mutex {
	if sessionID == "" {
		return &sync.Mutex{}
	}
	val, _ := s.appendLocks.LoadOrStore(sessionID, &sync.Mutex{})
	return val.(*sync.Mutex)
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

func (s *BadgerStore) BackendName() string {
	return "badger"
}

func (s *BadgerStore) Ping(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("badger store is not initialized")
	}
	return s.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte("sys:version"))
		if err == badger.ErrKeyNotFound {
			return nil
		}
		return err
	})
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

func (s *BadgerStore) metricKey(orgID string, metric string) []byte {
	return []byte(fmt.Sprintf("org:%s:m:%s", orgID, metric))
}

func (s *BadgerStore) metricTSKey(orgID string, metric string, minuteBucketStartMs int64) []byte {
	// Minute bucket start as 13-digit ms to preserve lexicographic ordering
	return []byte(fmt.Sprintf("org:%s:ts:%s:%013d", orgID, metric, minuteBucketStartMs))
}

// IncrementOrgMetric increments the org counter and a 1-minute timeseries bucket.
func (s *BadgerStore) IncrementOrgMetric(orgID string, metric string, delta uint64) error {
	now := time.Now().UnixMilli()
	minuteStart := (now / 60000) * 60000

	return s.db.Update(func(txn *badger.Txn) error {
		// total counter
		key := s.metricKey(orgID, metric)
		current := uint64(0)
		if item, err := txn.Get(key); err == nil {
			_ = item.Value(func(v []byte) error {
				if len(v) == 8 {
					current = binary.BigEndian.Uint64(v)
				}
				return nil
			})
		}
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, current+delta)
		if err := txn.Set(key, buf); err != nil {
			return err
		}

		// minute bucket
		tsKey := s.metricTSKey(orgID, metric, minuteStart)
		tsCurrent := uint64(0)
		if item, err := txn.Get(tsKey); err == nil {
			_ = item.Value(func(v []byte) error {
				if len(v) == 8 {
					tsCurrent = binary.BigEndian.Uint64(v)
				}
				return nil
			})
		}
		tsBuf := make([]byte, 8)
		binary.BigEndian.PutUint64(tsBuf, tsCurrent+delta)
		return txn.Set(tsKey, tsBuf)
	})
}

type MetricPoint struct {
	TimestampMs int64  `json:"ts"`
	Value       uint64 `json:"value"`
}

func (s *BadgerStore) GetOrgMetricTimeseries(orgID string, metric string, fromMs int64, toMs int64, bucketMs int64) ([]MetricPoint, error) {
	if bucketMs <= 0 {
		bucketMs = 60000
	}
	// stored in minute buckets
	fromMinute := (fromMs / 60000) * 60000
	toMinute := (toMs / 60000) * 60000

	// Aggregate by requested bucket
	agg := map[int64]uint64{}
	prefix := []byte(fmt.Sprintf("org:%s:ts:%s:", orgID, metric))

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		startKey := s.metricTSKey(orgID, metric, fromMinute)
		for it.Seek(startKey); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			k := string(item.Key())
			parts := strings.Split(k, ":")
			if len(parts) < 5 {
				continue
			}
			tsStr := parts[len(parts)-1]
			ts, err := strconv.ParseInt(tsStr, 10, 64)
			if err != nil {
				continue
			}
			if ts > toMinute {
				break
			}
			val := uint64(0)
			_ = item.Value(func(v []byte) error {
				if len(v) == 8 {
					val = binary.BigEndian.Uint64(v)
				}
				return nil
			})
			bucketStart := (ts / bucketMs) * bucketMs
			agg[bucketStart] += val
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	points := make([]MetricPoint, 0, len(agg))
	for ts, v := range agg {
		points = append(points, MetricPoint{TimestampMs: ts, Value: v})
	}
	sort.Slice(points, func(i, j int) bool { return points[i].TimestampMs < points[j].TimestampMs })
	return points, nil
}

func (s *BadgerStore) IncOrgRequestBreakdown(orgID string, endpoint string, status int, delta uint64) error {
	// Endpoint is stored verbatim; expected to be mux pattern (stable).
	key := []byte(fmt.Sprintf("org:%s:req:%s:%d", orgID, endpoint, status))
	return s.db.Update(func(txn *badger.Txn) error {
		cur := uint64(0)
		if item, err := txn.Get(key); err == nil {
			_ = item.Value(func(v []byte) error {
				if len(v) == 8 {
					cur = binary.BigEndian.Uint64(v)
				}
				return nil
			})
		}
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, cur+delta)
		return txn.Set(key, buf)
	})
}

type UsageBreakdown struct {
	ByEndpoint map[string]uint64            `json:"by_endpoint"`
	ByStatus   map[string]uint64            `json:"by_status"`
	Raw        map[string]map[string]uint64 `json:"raw"`
}

func (s *BadgerStore) GetOrgUsageBreakdown(orgID string) (*UsageBreakdown, error) {
	prefix := []byte(fmt.Sprintf("org:%s:req:", orgID))
	byEndpoint := map[string]uint64{}
	byStatus := map[string]uint64{}
	raw := map[string]map[string]uint64{}

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			k := strings.TrimPrefix(string(it.Item().Key()), string(prefix))
			// key format: {endpoint}:{status}
			lastColon := strings.LastIndex(k, ":")
			if lastColon <= 0 {
				continue
			}
			endpoint := k[:lastColon]
			status := k[lastColon+1:]
			val := uint64(0)
			_ = it.Item().Value(func(v []byte) error {
				if len(v) == 8 {
					val = binary.BigEndian.Uint64(v)
				}
				return nil
			})
			byEndpoint[endpoint] += val
			byStatus[status] += val
			if raw[endpoint] == nil {
				raw[endpoint] = map[string]uint64{}
			}
			raw[endpoint][status] += val
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &UsageBreakdown{ByEndpoint: byEndpoint, ByStatus: byStatus, Raw: raw}, nil
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

	// Maintain a linear, tamper-evident hash chain per session.
	// Under heavy concurrent writes to the same session, serialize Append to avoid Badger transaction conflicts
	// and ensure the chain head is updated deterministically.
	lock := s.appendLock(entry.SessionID)
	lock.Lock()
	defer lock.Unlock()

	var lastErr error
	for attempt := 0; attempt < 6; attempt++ {
		lastErr = s.db.Update(func(txn *badger.Txn) error {
			if err := txn.Set(key, val); err != nil {
				return err
			}

			// Tamper-evident hash chain:
			// head := sha256(prev_head || entry_json)
			headKey := []byte(fmt.Sprintf("s:%s:hchain:head", entry.SessionID))
			prev := []byte{}
			if item, err := txn.Get(headKey); err == nil {
				_ = item.Value(func(v []byte) error {
					prev = make([]byte, len(v))
					copy(prev, v)
					return nil
				})
			}
			sum := sha256.Sum256(append(prev, val...))
			hashKey := []byte(fmt.Sprintf("s:%s:hchain:%020d", entry.SessionID, now.UnixNano()))
			if err := txn.Set(hashKey, sum[:]); err != nil {
				return err
			}
			if err := txn.Set(headKey, sum[:]); err != nil {
				return err
			}
			return s.bumpSessionVersionTxn(txn, entry.SessionID)
		})
		if lastErr == nil {
			return nil
		}
		if lastErr == badger.ErrConflict {
			time.Sleep(time.Duration(attempt+1) * 2 * time.Millisecond)
			continue
		}
		return lastErr
	}
	return lastErr
}

func (s *BadgerStore) AppendOrgActivity(orgID string, entry JournalEntry) error {
	now := time.Now()
	val, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	key := []byte(fmt.Sprintf("org:%s:activity:%020d:%06s", orgID, now.UnixNano(), hex.EncodeToString([]byte(entry.Type))[:6]))
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, val)
	})
}

func (s *BadgerStore) ListOrgActivityPage(orgID string, cursor string, limit int) ([]JournalEntry, string, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	prefix := []byte(fmt.Sprintf("org:%s:activity:", orgID))
	out := make([]JournalEntry, 0, limit)
	var next string

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		opts.Reverse = true
		it := txn.NewIterator(opts)
		defer it.Close()

		seek := []byte(cursor)
		if cursor == "" {
			seek = append(prefix, 0xff)
		}
		for it.Seek(seek); it.Valid(); it.Next() {
			item := it.Item()
			if !strings.HasPrefix(string(item.Key()), string(prefix)) {
				break
			}
			var e JournalEntry
			if err := item.Value(func(v []byte) error { return json.Unmarshal(v, &e) }); err != nil {
				continue
			}
			out = append(out, e)
			if len(out) >= limit {
				it.Next()
				if it.Valid() && strings.HasPrefix(string(it.Item().Key()), string(prefix)) {
					next = string(it.Item().Key())
				}
				break
			}
		}
		return nil
	})
	return out, next, err
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

func (s *BadgerStore) GetSessionHistoryAfter(sessionID string, afterTimestamp int64) ([]JournalEntry, error) {
	history, err := s.GetSessionHistory(sessionID)
	if err != nil {
		return nil, err
	}
	if afterTimestamp <= 0 {
		return history, nil
	}
	filtered := make([]JournalEntry, 0, len(history))
	for _, entry := range history {
		if entry.Timestamp > afterTimestamp {
			filtered = append(filtered, entry)
		}
	}
	return filtered, nil
}

type ExportJob struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	OrgID       string `json:"org_id"`
	Format      string `json:"format"` // csv|pdf
	Status      string `json:"status"` // queued|running|done|error
	Error       string `json:"error,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	CompletedAt int64  `json:"completed_at,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Filename    string `json:"filename,omitempty"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`
}

func (s *BadgerStore) exportJobKey(sessionID string, id string) []byte {
	return []byte(fmt.Sprintf("s:%s:exportjob:%s", sessionID, id))
}
func (s *BadgerStore) exportJobDataKey(sessionID string, id string) []byte {
	return []byte(fmt.Sprintf("s:%s:exportjob:%s:data", sessionID, id))
}

func (s *BadgerStore) SaveExportJob(job *ExportJob) error {
	val, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(s.exportJobKey(job.SessionID, job.ID), val)
	})
}

func (s *BadgerStore) GetExportJob(sessionID string, id string) (*ExportJob, error) {
	var job ExportJob
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(s.exportJobKey(sessionID, id))
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error { return json.Unmarshal(v, &job) })
	})
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *BadgerStore) SaveExportJobResult(sessionID string, id string, contentType string, filename string, data []byte, errMsg string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get(s.exportJobKey(sessionID, id))
		if err != nil {
			return err
		}
		var job ExportJob
		if err := item.Value(func(v []byte) error { return json.Unmarshal(v, &job) }); err != nil {
			return err
		}
		job.CompletedAt = time.Now().UnixMilli()
		if errMsg != "" {
			job.Status = "error"
			job.Error = errMsg
		} else {
			job.Status = "done"
			job.ContentType = contentType
			job.Filename = filename
			job.SizeBytes = int64(len(data))
			if err := txn.Set(s.exportJobDataKey(sessionID, id), data); err != nil {
				return err
			}
		}
		val, err := json.Marshal(job)
		if err != nil {
			return err
		}
		return txn.Set(s.exportJobKey(sessionID, id), val)
	})
}

func (s *BadgerStore) GetExportJobData(sessionID string, id string) ([]byte, error) {
	var out []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(s.exportJobDataKey(sessionID, id))
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error {
			out = make([]byte, len(v))
			copy(out, v)
			return nil
		})
	})
	return out, err
}

func (s *BadgerStore) ListExportJobs(sessionID string, limit int) ([]ExportJob, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	prefix := []byte(fmt.Sprintf("s:%s:exportjob:", sessionID))
	out := []ExportJob{}
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			k := string(it.Item().Key())
			if strings.HasSuffix(k, ":data") {
				continue
			}
			var job ExportJob
			if err := it.Item().Value(func(v []byte) error { return json.Unmarshal(v, &job) }); err != nil {
				continue
			}
			out = append(out, job)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// GetSessionHistoryPage returns a filtered, newest-first slice of session history plus a cursor.
// Cursor is an opaque Badger key string previously returned as nextCursor.
func (s *BadgerStore) GetSessionHistoryPage(sessionID string, cursor string, limit int, fromMs int64, toMs int64, typ string, q string) ([]JournalEntry, string, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	prefix := []byte(fmt.Sprintf("s:%s:h:", sessionID))
	var nextCursor string
	out := make([]JournalEntry, 0, limit)

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		opts.Reverse = true
		it := txn.NewIterator(opts)
		defer it.Close()

		seek := []byte(cursor)
		if cursor == "" {
			seek = append(prefix, 0xff)
		}

		for it.Seek(seek); it.Valid(); it.Next() {
			item := it.Item()
			k := item.Key()
			if !strings.HasPrefix(string(k), string(prefix)) {
				break
			}
			var entry JournalEntry
			raw := []byte(nil)
			err := item.Value(func(v []byte) error {
				raw = make([]byte, len(v))
				copy(raw, v)
				return json.Unmarshal(v, &entry)
			})
			if err != nil {
				continue
			}
			if fromMs != 0 && entry.Timestamp < fromMs {
				continue
			}
			if toMs != 0 && entry.Timestamp > toMs {
				continue
			}
			if typ != "" && string(entry.Type) != typ {
				continue
			}
			if q != "" && !strings.Contains(strings.ToLower(string(raw)), q) {
				continue
			}
			out = append(out, entry)
			if len(out) >= limit {
				it.Next()
				if it.Valid() && strings.HasPrefix(string(it.Item().Key()), string(prefix)) {
					nextCursor = string(it.Item().Key())
				}
				break
			}
		}
		return nil
	})

	return out, nextCursor, err
}

// SaveConfig persists session-specific settings
func (s *BadgerStore) SaveConfig(sessionID string, config interface{}) error {
	val, err := json.Marshal(config)
	if err != nil {
		return err
	}
	key := []byte(fmt.Sprintf("s:%s:c", sessionID))
	return s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(key, val); err != nil {
			return err
		}
		return s.bumpSessionVersionTxn(txn, sessionID)
	})
}

type SessionConfig struct {
	Schema          string `json:"schema"`
	EnforcementMode string `json:"enforcement_mode"` // "strict" or "warn"
}

type APIKey struct {
	// NOTE: For security, Velarix does not persist raw API keys for new keys.
	// The `Key` field may be present for legacy records and in "issued" responses only.
	Key string `json:"key,omitempty"`

	// Stable identifier for this key (sha256 hex of raw key).
	ID string `json:"id,omitempty"`

	// Persisted hash of the raw key (sha256 hex). Used for auth lookups.
	KeyHash string `json:"key_hash,omitempty"`

	// Redacted display helpers for UI/logging.
	KeyPrefix string `json:"key_prefix,omitempty"`
	KeyLast4  string `json:"key_last4,omitempty"`

	Label      string   `json:"label"`
	CreatedAt  int64    `json:"created_at"`
	LastUsedAt int64    `json:"last_used_at"`
	ExpiresAt  int64    `json:"expires_at"` // Add expiration for key rotation
	IsRevoked  bool     `json:"is_revoked"`
	Scopes     []string `json:"scopes,omitempty"` // read|write|export|admin
}

type Organization struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	CreatedAt   int64  `json:"created_at"`
	IsSuspended bool   `json:"is_suspended"`

	// Optional org settings (backwards compatible)
	Settings map[string]interface{} `json:"settings,omitempty"`
}

type User struct {
	Email          string   `json:"email"`
	HashedPassword string   `json:"hashed_password"`
	OrgID          string   `json:"org_id"`
	Role           string   `json:"role"` // "admin" or "member"
	TokenVersion   int64    `json:"token_version,omitempty"`
	Keys           []APIKey `json:"keys"`
	ResetToken     string   `json:"reset_token,omitempty"`
	ResetExpiry    int64    `json:"reset_expiry,omitempty"`

	// Optional console state (backwards compatible)
	Onboarding map[string]bool `json:"onboarding,omitempty"`
}

func (u *User) GetTokenVersion() int64 {
	if u == nil {
		return 0
	}
	return u.TokenVersion
}

type OrgSessionMeta struct {
	ID             string `json:"id"`
	CreatedAt      int64  `json:"created_at"`
	LastActivityAt int64  `json:"last_activity_at"`
	FactCount      int    `json:"fact_count"`
}

type Notification struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	CreatedAt int64  `json:"created_at"`
	ReadAt    int64  `json:"read_at,omitempty"`
}

type AccessLogEntry struct {
	ID         string `json:"id"`
	ActorID    string `json:"actor_id"`
	ActorRole  string `json:"actor_role,omitempty"`
	Method     string `json:"method"`
	Pattern    string `json:"pattern,omitempty"`
	Path       string `json:"path"`
	Status     int    `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	TraceID    string `json:"trace_id,omitempty"`
	IP         string `json:"ip,omitempty"`
	UserAgent  string `json:"user_agent,omitempty"`
	CreatedAt  int64  `json:"created_at"`
}

type Integration struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Kind      string                 `json:"kind"`
	Enabled   bool                   `json:"enabled"`
	Config    map[string]interface{} `json:"config,omitempty"`
	CreatedAt int64                  `json:"created_at"`
	UpdatedAt int64                  `json:"updated_at"`
}

type BillingSubscription struct {
	Plan                 string            `json:"plan"`
	Status               string            `json:"status"`
	BillingEmail         string            `json:"billing_email"`
	StripeCustomerID     string            `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID string            `json:"stripe_subscription_id,omitempty"`
	CurrentPeriodEnd     int64             `json:"current_period_end,omitempty"`
	Seats                int               `json:"seats,omitempty"`
	Features             map[string]bool   `json:"features,omitempty"`
	Metadata             map[string]string `json:"metadata,omitempty"`
	UpdatedAt            int64             `json:"updated_at"`
}

type Invitation struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	Token      string `json:"token"`
	CreatedAt  int64  `json:"created_at"`
	ExpiresAt  int64  `json:"expires_at"`
	AcceptedAt int64  `json:"accepted_at,omitempty"`
	RevokedAt  int64  `json:"revoked_at,omitempty"`
}

type SupportTicket struct {
	ID        string `json:"id"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Status    string `json:"status"` // open|closed
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
	CreatedBy string `json:"created_by"`
}

type Policy struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Enabled   bool                   `json:"enabled"`
	Rules     map[string]interface{} `json:"rules,omitempty"`
	CreatedAt int64                  `json:"created_at"`
	UpdatedAt int64                  `json:"updated_at"`
}

func (s *BadgerStore) sessionIndexKey(orgID, sessionID string) []byte {
	return []byte(fmt.Sprintf("org:%s:sessions:%s", orgID, sessionID))
}

// UpsertOrgSessionIndex ensures a session appears in the org catalog.
func (s *BadgerStore) UpsertOrgSessionIndex(orgID, sessionID string, createdAt int64) error {
	return s.db.Update(func(txn *badger.Txn) error {
		key := s.sessionIndexKey(orgID, sessionID)
		var meta OrgSessionMeta
		item, err := txn.Get(key)
		if err == nil {
			_ = item.Value(func(v []byte) error { return json.Unmarshal(v, &meta) })
		}
		if meta.ID == "" {
			meta = OrgSessionMeta{
				ID:             sessionID,
				CreatedAt:      createdAt,
				LastActivityAt: createdAt,
				FactCount:      0,
			}
		}
		val, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		return txn.Set(key, val)
	})
}

func (s *BadgerStore) TouchOrgSession(orgID, sessionID string, factDelta int) error {
	now := time.Now().UnixMilli()
	return s.db.Update(func(txn *badger.Txn) error {
		key := s.sessionIndexKey(orgID, sessionID)
		var meta OrgSessionMeta
		if item, err := txn.Get(key); err == nil {
			_ = item.Value(func(v []byte) error { return json.Unmarshal(v, &meta) })
		}
		if meta.ID == "" {
			meta.ID = sessionID
			meta.CreatedAt = now
		}
		meta.LastActivityAt = now
		meta.FactCount += factDelta
		if meta.FactCount < 0 {
			meta.FactCount = 0
		}
		val, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		return txn.Set(key, val)
	})
}

func (s *BadgerStore) ListOrgSessions(orgID string, cursor string, limit int) ([]OrgSessionMeta, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	prefix := []byte(fmt.Sprintf("org:%s:sessions:", orgID))
	all := []OrgSessionMeta{}
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			var meta OrgSessionMeta
			if err := item.Value(func(v []byte) error { return json.Unmarshal(v, &meta) }); err != nil {
				continue
			}
			if meta.ID != "" {
				all = append(all, meta)
			}
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].LastActivityAt == all[j].LastActivityAt {
			return all[i].ID > all[j].ID
		}
		return all[i].LastActivityAt > all[j].LastActivityAt
	})

	start := 0
	if cursor != "" {
		for i := range all {
			c := fmt.Sprintf("%d:%s", all[i].LastActivityAt, all[i].ID)
			if c == cursor {
				start = i + 1
				break
			}
		}
	}
	end := start + limit
	if end > len(all) {
		end = len(all)
	}
	page := all[start:end]
	nextCursor := ""
	if end < len(all) && len(page) > 0 {
		last := page[len(page)-1]
		nextCursor = fmt.Sprintf("%d:%s", last.LastActivityAt, last.ID)
	}
	return page, nextCursor, nil
}

func (s *BadgerStore) SaveNotification(orgID string, n *Notification) error {
	val, err := json.Marshal(n)
	if err != nil {
		return err
	}
	key := []byte(fmt.Sprintf("org:%s:notif:%020d:%s", orgID, n.CreatedAt, n.ID))
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, val)
	})
}

func (s *BadgerStore) ListNotifications(orgID string, cursor string, limit int) ([]Notification, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	prefix := []byte(fmt.Sprintf("org:%s:notif:", orgID))
	var nextCursor string
	out := make([]Notification, 0, limit)

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		opts.Reverse = true
		it := txn.NewIterator(opts)
		defer it.Close()

		var seek []byte
		if cursor != "" {
			seek = []byte(cursor)
		} else {
			seek = append(prefix, 0xff)
		}

		for it.Seek(seek); it.Valid(); it.Next() {
			item := it.Item()
			k := item.Key()
			if !strings.HasPrefix(string(k), string(prefix)) {
				break
			}
			var n Notification
			if err := item.Value(func(v []byte) error { return json.Unmarshal(v, &n) }); err != nil {
				continue
			}
			out = append(out, n)
			if len(out) >= limit {
				it.Next()
				if it.Valid() {
					nextCursor = string(it.Item().Key())
				}
				break
			}
		}
		return nil
	})

	return out, nextCursor, err
}

func (s *BadgerStore) MarkNotificationRead(orgID string, notificationID string) error {
	prefix := []byte(fmt.Sprintf("org:%s:notif:", orgID))
	return s.db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			var n Notification
			if err := item.Value(func(v []byte) error { return json.Unmarshal(v, &n) }); err != nil {
				continue
			}
			if n.ID != notificationID {
				continue
			}
			if n.ReadAt == 0 {
				n.ReadAt = time.Now().UnixMilli()
			}
			val, err := json.Marshal(n)
			if err != nil {
				return err
			}
			return txn.Set(item.Key(), val)
		}
		return badger.ErrKeyNotFound
	})
}

func (s *BadgerStore) AppendAccessLog(orgID string, e AccessLogEntry) error {
	now := time.Now()
	if e.CreatedAt == 0 {
		e.CreatedAt = now.UnixMilli()
	}
	val, err := json.Marshal(e)
	if err != nil {
		return err
	}
	methodTag := hex.EncodeToString([]byte(e.Method))
	if len(methodTag) < 6 {
		methodTag += strings.Repeat("0", 6-len(methodTag))
	}
	key := []byte(fmt.Sprintf("org:%s:access:%020d:%06s", orgID, now.UnixNano(), methodTag[:6]))
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, val)
	})
}

func (s *BadgerStore) ListAccessLogsPage(orgID string, cursor string, limit int) ([]AccessLogEntry, string, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	prefix := []byte(fmt.Sprintf("org:%s:access:", orgID))
	out := make([]AccessLogEntry, 0, limit)
	var next string

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		opts.Reverse = true
		it := txn.NewIterator(opts)
		defer it.Close()

		seek := []byte(cursor)
		if cursor == "" {
			seek = append(prefix, 0xff)
		}
		for it.Seek(seek); it.Valid(); it.Next() {
			item := it.Item()
			if !strings.HasPrefix(string(item.Key()), string(prefix)) {
				break
			}
			var e AccessLogEntry
			if err := item.Value(func(v []byte) error { return json.Unmarshal(v, &e) }); err != nil {
				continue
			}
			out = append(out, e)
			if len(out) >= limit {
				it.Next()
				if it.Valid() && strings.HasPrefix(string(it.Item().Key()), string(prefix)) {
					next = string(it.Item().Key())
				}
				break
			}
		}
		return nil
	})
	return out, next, err
}

func (s *BadgerStore) integrationKey(orgID, id string) []byte {
	return []byte(fmt.Sprintf("org:%s:integration:%s", orgID, id))
}

func (s *BadgerStore) SaveIntegration(orgID string, integ *Integration) error {
	val, err := json.Marshal(integ)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(s.integrationKey(orgID, integ.ID), val)
	})
}

func (s *BadgerStore) GetIntegration(orgID, id string) (*Integration, error) {
	key := s.integrationKey(orgID, id)
	var integ Integration
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error { return json.Unmarshal(v, &integ) })
	})
	if err != nil {
		return nil, err
	}
	return &integ, nil
}

func (s *BadgerStore) DeleteIntegration(orgID, id string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(s.integrationKey(orgID, id))
	})
}

func (s *BadgerStore) ListIntegrations(orgID string) ([]Integration, error) {
	prefix := []byte(fmt.Sprintf("org:%s:integration:", orgID))
	out := []Integration{}
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			var integ Integration
			if err := item.Value(func(v []byte) error { return json.Unmarshal(v, &integ) }); err != nil {
				continue
			}
			out = append(out, integ)
		}
		return nil
	})
	return out, err
}

func (s *BadgerStore) billingKey(orgID string) []byte {
	return []byte(fmt.Sprintf("org:%s:billing", orgID))
}

func (s *BadgerStore) GetBilling(orgID string) (*BillingSubscription, error) {
	key := s.billingKey(orgID)
	var sub BillingSubscription
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error { return json.Unmarshal(v, &sub) })
	})
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *BadgerStore) SaveBilling(orgID string, sub *BillingSubscription) error {
	val, err := json.Marshal(sub)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(s.billingKey(orgID), val)
	})
}

func (s *BadgerStore) ListOrgUsers(orgID string) ([]string, error) {
	prefix := []byte(fmt.Sprintf("org:%s:user:", orgID))
	out := []string{}
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			k := string(it.Item().Key())
			email := strings.TrimPrefix(k, string(prefix))
			if email != "" {
				out = append(out, email)
			}
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

func (s *BadgerStore) inviteKey(orgID, id string) []byte {
	return []byte(fmt.Sprintf("org:%s:invite:%s", orgID, id))
}

func (s *BadgerStore) inviteTokenKey(token string) []byte {
	return []byte(fmt.Sprintf("invite:token:%s", token))
}

func (s *BadgerStore) SaveInvitation(orgID string, inv *Invitation) error {
	val, err := json.Marshal(inv)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(s.inviteKey(orgID, inv.ID), val); err != nil {
			return err
		}
		return txn.Set(s.inviteTokenKey(inv.Token), []byte(fmt.Sprintf("%s:%s", orgID, inv.ID)))
	})
}

func (s *BadgerStore) ListInvitations(orgID string) ([]Invitation, error) {
	prefix := []byte(fmt.Sprintf("org:%s:invite:", orgID))
	out := []Invitation{}
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			var inv Invitation
			item := it.Item()
			if err := item.Value(func(v []byte) error { return json.Unmarshal(v, &inv) }); err != nil {
				continue
			}
			out = append(out, inv)
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, err
}

func (s *BadgerStore) GetInvitationByToken(token string) (string, *Invitation, error) {
	var orgID string
	var id string
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(s.inviteTokenKey(token))
		if err != nil {
			return err
		}
		var ref string
		if err := item.Value(func(v []byte) error { ref = string(v); return nil }); err != nil {
			return err
		}
		parts := strings.SplitN(ref, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("corrupt invite token ref")
		}
		orgID, id = parts[0], parts[1]
		return nil
	})
	if err != nil {
		return "", nil, err
	}
	inv, err := s.GetInvitation(orgID, id)
	return orgID, inv, err
}

func (s *BadgerStore) GetInvitation(orgID, id string) (*Invitation, error) {
	key := s.inviteKey(orgID, id)
	var inv Invitation
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error { return json.Unmarshal(v, &inv) })
	})
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (s *BadgerStore) UpdateInvitation(orgID string, inv *Invitation) error {
	val, err := json.Marshal(inv)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(s.inviteKey(orgID, inv.ID), val)
	})
}

func (s *BadgerStore) ticketKey(orgID, id string) []byte {
	return []byte(fmt.Sprintf("org:%s:ticket:%s", orgID, id))
}

func (s *BadgerStore) SaveTicket(orgID string, t *SupportTicket) error {
	val, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(s.ticketKey(orgID, t.ID), val)
	})
}

func (s *BadgerStore) ListTickets(orgID string) ([]SupportTicket, error) {
	prefix := []byte(fmt.Sprintf("org:%s:ticket:", orgID))
	out := []SupportTicket{}
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			var t SupportTicket
			if err := it.Item().Value(func(v []byte) error { return json.Unmarshal(v, &t) }); err != nil {
				continue
			}
			out = append(out, t)
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out, err
}

func (s *BadgerStore) GetTicket(orgID, id string) (*SupportTicket, error) {
	var t SupportTicket
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(s.ticketKey(orgID, id))
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error { return json.Unmarshal(v, &t) })
	})
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *BadgerStore) policyKey(orgID, id string) []byte {
	return []byte(fmt.Sprintf("org:%s:policy:%s", orgID, id))
}

func (s *BadgerStore) SavePolicy(orgID string, p *Policy) error {
	val, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(s.policyKey(orgID, p.ID), val)
	})
}

func (s *BadgerStore) ListPolicies(orgID string) ([]Policy, error) {
	prefix := []byte(fmt.Sprintf("org:%s:policy:", orgID))
	out := []Policy{}
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			var p Policy
			if err := it.Item().Value(func(v []byte) error { return json.Unmarshal(v, &p) }); err != nil {
				continue
			}
			out = append(out, p)
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out, err
}

func (s *BadgerStore) DeletePolicy(orgID, id string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(s.policyKey(orgID, id))
	})
}

func (s *BadgerStore) GetPolicy(orgID, id string) (*Policy, error) {
	var p Policy
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(s.policyKey(orgID, id))
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error { return json.Unmarshal(v, &p) })
	})
	if err != nil {
		return nil, err
	}
	return &p, nil
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
		if err := txn.Set([]byte("s:"+sessionID+":org"), []byte(orgID)); err != nil {
			return err
		}
		return s.bumpSessionVersionTxn(txn, sessionID)
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
			keyHash := k.KeyHash
			if keyHash == "" {
				keyHash = k.ID
			}

			if !k.IsRevoked {
				// Prefer hashed index for all keys that have it.
				if keyHash != "" {
					if err := txn.Set([]byte("kh:"+keyHash), []byte(user.Email)); err != nil {
						return err
					}
				}
				// Backwards-compatible plaintext index for legacy keys only.
				if k.Key != "" {
					if err := txn.Set([]byte("k:"+k.Key), []byte(user.Email)); err != nil {
						return err
					}
				}
			} else {
				if keyHash != "" {
					_ = txn.Delete([]byte("kh:" + keyHash))
				}
				if k.Key != "" {
					// Ensure revoked legacy keys are removed from the lookup map
					_ = txn.Delete([]byte("k:" + k.Key))
				}
			}
		}

		// Org user index (best-effort)
		_ = txn.Set([]byte(fmt.Sprintf("org:%s:user:%s", user.OrgID, user.Email)), []byte{1})
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
		if err := txn.Set(key, val); err != nil {
			return err
		}
		return s.bumpSessionVersionTxn(txn, sessionID)
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

// GetAPIKeyOwnerByHash returns the email of the user who owns the given key hash (sha256 hex).
func (s *BadgerStore) GetAPIKeyOwnerByHash(keyHash string) ([]byte, error) {
	var email []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("kh:" + keyHash))
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

// SaveAPIKeyHash stores a new API key hash owner mapping.
func (s *BadgerStore) SaveAPIKeyHash(keyHash, email string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte("kh:"+keyHash), []byte(email))
	})
}

func (s *BadgerStore) DeleteAPIKeyHash(keyHash string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte("kh:" + keyHash))
	})
}

func (s *BadgerStore) DeleteAPIKey(key string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte("k:" + key))
	})
}

type IdempotencyRecord struct {
	Status      int               `json:"status"`
	ContentType string            `json:"content_type,omitempty"`
	Body        []byte            `json:"body,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	CreatedAt   int64             `json:"created_at"`
}

func (s *BadgerStore) idempotencyKey(orgID string, keyHash string) []byte {
	return []byte(fmt.Sprintf("org:%s:idem:%s", orgID, keyHash))
}

func (s *BadgerStore) GetIdempotency(orgID string, keyHash string, maxAge time.Duration) (*IdempotencyRecord, error) {
	var rec IdempotencyRecord
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(s.idempotencyKey(orgID, keyHash))
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error { return json.Unmarshal(v, &rec) })
	})
	if err != nil {
		return nil, err
	}
	if maxAge > 0 && rec.CreatedAt > 0 {
		if time.Since(time.UnixMilli(rec.CreatedAt)) > maxAge {
			_ = s.DeleteIdempotency(orgID, keyHash)
			return nil, badger.ErrKeyNotFound
		}
	}
	return &rec, nil
}

func (s *BadgerStore) SaveIdempotency(orgID string, keyHash string, rec *IdempotencyRecord) error {
	if rec.CreatedAt == 0 {
		rec.CreatedAt = time.Now().UnixMilli()
	}
	val, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(s.idempotencyKey(orgID, keyHash), val)
	})
}

func (s *BadgerStore) DeleteIdempotency(orgID string, keyHash string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(s.idempotencyKey(orgID, keyHash))
	})
}

func (s *BadgerStore) GetSessionHistoryChainHead(sessionID string) (string, error) {
	headKey := []byte(fmt.Sprintf("s:%s:hchain:head", sessionID))
	var out []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(headKey)
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error {
			out = make([]byte, len(v))
			copy(out, v)
			return nil
		})
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(out), nil
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

				switch entry.Type {
				case EventAssert:
					_ = engine.AssertFact(entry.Fact)
				case EventInvalidate:
					_ = engine.InvalidateRoot(entry.FactID)
				case EventRetract:
					reason := ""
					if entry.Payload != nil {
						if v, ok := entry.Payload["reason"].(string); ok {
							reason = v
						}
					}
					_ = engine.RetractFact(entry.FactID, reason)
				case EventReview:
					status := ""
					reason := ""
					reviewedAt := entry.Timestamp
					if entry.Payload != nil {
						if v, ok := entry.Payload["status"].(string); ok {
							status = v
						}
						if v, ok := entry.Payload["reason"].(string); ok {
							reason = v
						}
						if v, ok := entry.Payload["reviewed_at"].(float64); ok {
							reviewedAt = int64(v)
						}
					}
					_ = engine.SetFactReview(entry.FactID, status, reason, reviewedAt)
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

func (s *BadgerStore) decisionKey(sessionID, decisionID string) []byte {
	return []byte(fmt.Sprintf("s:%s:decision:%s", sessionID, decisionID))
}

func (s *BadgerStore) decisionOrgIndexKey(orgID, sessionID, decisionID string) []byte {
	return []byte(fmt.Sprintf("org:%s:decision:%s:%s", orgID, sessionID, decisionID))
}

func (s *BadgerStore) decisionDepsPrefix(sessionID, decisionID string) []byte {
	return []byte(fmt.Sprintf("s:%s:decisiondeps:%s:", sessionID, decisionID))
}

func (s *BadgerStore) decisionCheckLatestKey(sessionID, decisionID string) []byte {
	return []byte(fmt.Sprintf("s:%s:decisioncheck:%s:latest", sessionID, decisionID))
}

func (s *BadgerStore) decisionCheckHistoryKey(sessionID, decisionID string, checkedAt int64) []byte {
	return []byte(fmt.Sprintf("s:%s:decisioncheck:%s:%020d", sessionID, decisionID, checkedAt))
}

func (s *BadgerStore) SaveDecision(decision *Decision) error {
	if decision == nil {
		return fmt.Errorf("decision is required")
	}
	if decision.SessionID == "" || decision.ID == "" {
		return fmt.Errorf("decision session_id and decision_id are required")
	}
	if decision.CreatedAt == 0 {
		decision.CreatedAt = time.Now().UnixMilli()
	}
	if decision.UpdatedAt == 0 {
		decision.UpdatedAt = decision.CreatedAt
	}
	val, err := json.Marshal(decision)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(s.decisionKey(decision.SessionID, decision.ID), val); err != nil {
			return err
		}
		if decision.OrgID != "" {
			if err := txn.Set(s.decisionOrgIndexKey(decision.OrgID, decision.SessionID, decision.ID), []byte(decision.Status)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *BadgerStore) GetDecision(sessionID string, decisionID string) (*Decision, error) {
	var decision Decision
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(s.decisionKey(sessionID, decisionID))
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error { return json.Unmarshal(v, &decision) })
	})
	if err != nil {
		return nil, err
	}
	return &decision, nil
}

func decisionMatchesFilter(decision Decision, filter DecisionListFilter) bool {
	if filter.Status != "" && decision.Status != filter.Status && decision.ExecutionStatus != filter.Status {
		return false
	}
	if filter.SubjectRef != "" && decision.SubjectRef != filter.SubjectRef {
		return false
	}
	if filter.FromMs != 0 && decision.CreatedAt < filter.FromMs {
		return false
	}
	if filter.ToMs != 0 && decision.CreatedAt > filter.ToMs {
		return false
	}
	return true
}

func (s *BadgerStore) listDecisionsByPrefix(prefix []byte, filter DecisionListFilter) ([]Decision, error) {
	out := []Decision{}
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			if strings.HasSuffix(string(item.Key()), ":latest") {
				continue
			}
			var decision Decision
			if err := item.Value(func(v []byte) error { return json.Unmarshal(v, &decision) }); err != nil {
				continue
			}
			if !decisionMatchesFilter(decision, filter) {
				continue
			}
			out = append(out, decision)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, nil
}

func (s *BadgerStore) ListSessionDecisions(sessionID string, filter DecisionListFilter) ([]Decision, error) {
	return s.listDecisionsByPrefix([]byte(fmt.Sprintf("s:%s:decision:", sessionID)), filter)
}

func (s *BadgerStore) ListOrgDecisions(orgID string, filter DecisionListFilter) ([]Decision, error) {
	prefix := []byte(fmt.Sprintf("org:%s:decision:", orgID))
	out := []Decision{}
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			parts := strings.Split(string(it.Item().Key()), ":")
			if len(parts) < 5 {
				continue
			}
			sessionID := parts[3]
			decisionID := parts[4]
			item, err := txn.Get(s.decisionKey(sessionID, decisionID))
			if err != nil {
				continue
			}
			var decision Decision
			if err := item.Value(func(v []byte) error { return json.Unmarshal(v, &decision) }); err != nil {
				continue
			}
			if !decisionMatchesFilter(decision, filter) {
				continue
			}
			out = append(out, decision)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, nil
}

func (s *BadgerStore) SaveDecisionDependencies(sessionID string, decisionID string, deps []DecisionDependency) error {
	prefix := s.decisionDepsPrefix(sessionID, decisionID)
	return s.db.Update(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			if err := txn.Delete(it.Item().KeyCopy(nil)); err != nil {
				return err
			}
		}
		for _, dep := range deps {
			dep.SessionID = sessionID
			dep.DecisionID = decisionID
			val, err := json.Marshal(dep)
			if err != nil {
				return err
			}
			key := append(prefix, []byte(dep.FactID)...)
			if err := txn.Set(key, val); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *BadgerStore) GetDecisionDependencies(sessionID string, decisionID string) ([]DecisionDependency, error) {
	prefix := s.decisionDepsPrefix(sessionID, decisionID)
	out := []DecisionDependency{}
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			var dep DecisionDependency
			if err := it.Item().Value(func(v []byte) error { return json.Unmarshal(v, &dep) }); err != nil {
				continue
			}
			out = append(out, dep)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FactID < out[j].FactID })
	return out, nil
}

func (s *BadgerStore) SaveDecisionCheck(sessionID string, decisionID string, check *DecisionCheck) error {
	if check == nil {
		return fmt.Errorf("check is required")
	}
	if check.CheckedAt == 0 {
		check.CheckedAt = time.Now().UnixMilli()
	}
	check.SessionID = sessionID
	check.DecisionID = decisionID
	val, err := json.Marshal(check)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(s.decisionCheckLatestKey(sessionID, decisionID), val); err != nil {
			return err
		}
		return txn.Set(s.decisionCheckHistoryKey(sessionID, decisionID, check.CheckedAt), val)
	})
}

func (s *BadgerStore) GetLatestDecisionCheck(sessionID string, decisionID string) (*DecisionCheck, error) {
	var check DecisionCheck
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(s.decisionCheckLatestKey(sessionID, decisionID))
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error { return json.Unmarshal(v, &check) })
	})
	if err != nil {
		return nil, err
	}
	return &check, nil
}

func (s *BadgerStore) searchDocumentKey(doc SearchDocument) []byte {
	return []byte(fmt.Sprintf("org:%s:search:%s:%s", doc.OrgID, doc.DocumentType, doc.ID))
}

func (s *BadgerStore) UpsertSearchDocuments(docs []SearchDocument) error {
	return s.db.Update(func(txn *badger.Txn) error {
		for _, doc := range docs {
			if doc.ID == "" || doc.OrgID == "" {
				continue
			}
			if doc.UpdatedAt == 0 {
				doc.UpdatedAt = time.Now().UnixMilli()
			}
			if doc.CreatedAt == 0 {
				doc.CreatedAt = doc.UpdatedAt
			}
			val, err := json.Marshal(doc)
			if err != nil {
				return err
			}
			if err := txn.Set(s.searchDocumentKey(doc), val); err != nil {
				return err
			}
		}
		return nil
	})
}

func searchDocumentMatches(doc SearchDocument, filter SearchDocumentsFilter) bool {
	if filter.DocumentType != "" && doc.DocumentType != filter.DocumentType {
		return false
	}
	if filter.Status != "" && doc.Status != filter.Status {
		return false
	}
	if filter.SubjectRef != "" && doc.SubjectRef != filter.SubjectRef {
		return false
	}
	if filter.FromMs != 0 && doc.UpdatedAt < filter.FromMs {
		return false
	}
	if filter.ToMs != 0 && doc.UpdatedAt > filter.ToMs {
		return false
	}
	if q := strings.TrimSpace(strings.ToLower(filter.Query)); q != "" {
		if !strings.Contains(strings.ToLower(doc.Title), q) &&
			!strings.Contains(strings.ToLower(doc.Body), q) &&
			!strings.Contains(strings.ToLower(doc.SubjectRef), q) &&
			!strings.Contains(strings.ToLower(doc.TargetRef), q) &&
			!strings.Contains(strings.ToLower(doc.DecisionID), q) &&
			!strings.Contains(strings.ToLower(doc.FactID), q) {
			return false
		}
	}
	return true
}

func orgSettingAsInt(org *Organization, key string, def int) int {
	if org == nil || org.Settings == nil {
		return def
	}
	raw, ok := org.Settings[key]
	if !ok {
		return def
	}
	switch v := raw.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case json.Number:
		n, err := v.Int64()
		if err == nil {
			return int(n)
		}
	}
	return def
}

func (s *BadgerStore) SearchDocuments(orgID string, filter SearchDocumentsFilter) ([]SearchDocument, string, error) {
	prefix := []byte(fmt.Sprintf("org:%s:search:", orgID))
	out := []SearchDocument{}
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			var doc SearchDocument
			if err := it.Item().Value(func(v []byte) error { return json.Unmarshal(v, &doc) }); err != nil {
				continue
			}
			if !searchDocumentMatches(doc, filter) {
				continue
			}
			out = append(out, doc)
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt == out[j].UpdatedAt {
			return out[i].ID > out[j].ID
		}
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	offset := 0
	if filter.Cursor != "" && strings.HasPrefix(filter.Cursor, "o:") {
		if n, err := strconv.Atoi(strings.TrimPrefix(filter.Cursor, "o:")); err == nil && n >= 0 {
			offset = n
		}
	}
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset > len(out) {
		offset = len(out)
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	next := ""
	if end < len(out) {
		next = fmt.Sprintf("o:%d", end)
	}
	return out[offset:end], next, nil
}

func (s *BadgerStore) EnforceRetention(now time.Time) (*RetentionReport, error) {
	type orgRetention struct {
		orgID        string
		activityDays int
		accessDays   int
		notifyDays   int
	}
	orgs := []orgRetention{}

	if err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte("o:")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			var org Organization
			if err := it.Item().Value(func(v []byte) error { return json.Unmarshal(v, &org) }); err != nil {
				continue
			}
			orgs = append(orgs, orgRetention{
				orgID:        org.ID,
				activityDays: orgSettingAsInt(&org, "retention_days_activity", 30),
				accessDays:   orgSettingAsInt(&org, "retention_days_access_logs", 30),
				notifyDays:   orgSettingAsInt(&org, "retention_days_notifications", 30),
			})
		}
		return nil
	}); err != nil {
		return nil, err
	}

	report := &RetentionReport{}
	for _, org := range orgs {
		activityCutoff := now.Add(-time.Duration(org.activityDays) * 24 * time.Hour).UnixMilli()
		accessCutoff := now.Add(-time.Duration(org.accessDays) * 24 * time.Hour).UnixMilli()
		notifyCutoff := now.Add(-time.Duration(org.notifyDays) * 24 * time.Hour).UnixMilli()
		err := s.db.Update(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			opts.PrefetchValues = true
			it := txn.NewIterator(opts)
			defer it.Close()

			deleteBefore := func(prefix []byte, cutoff int64, decode func([]byte) (int64, error), count *int) error {
				for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
					ts := int64(0)
					if err := it.Item().Value(func(v []byte) error {
						decoded, decodeErr := decode(v)
						if decodeErr != nil {
							return decodeErr
						}
						ts = decoded
						return nil
					}); err != nil {
						continue
					}
					if ts > 0 && ts < cutoff {
						if err := txn.Delete(it.Item().KeyCopy(nil)); err != nil {
							return err
						}
						*count = *count + 1
					}
				}
				return nil
			}

			if err := deleteBefore([]byte(fmt.Sprintf("org:%s:activity:", org.orgID)), activityCutoff, func(v []byte) (int64, error) {
				var e JournalEntry
				if err := json.Unmarshal(v, &e); err != nil {
					return 0, err
				}
				return e.Timestamp, nil
			}, &report.ActivityDeleted); err != nil {
				return err
			}
			if err := deleteBefore([]byte(fmt.Sprintf("org:%s:access:", org.orgID)), accessCutoff, func(v []byte) (int64, error) {
				var e AccessLogEntry
				if err := json.Unmarshal(v, &e); err != nil {
					return 0, err
				}
				return e.CreatedAt, nil
			}, &report.AccessLogsDeleted); err != nil {
				return err
			}
			if err := deleteBefore([]byte(fmt.Sprintf("org:%s:notif:", org.orgID)), notifyCutoff, func(v []byte) (int64, error) {
				var n Notification
				if err := json.Unmarshal(v, &n); err != nil {
					return 0, err
				}
				return n.CreatedAt, nil
			}, &report.NotificationsDeleted); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return report, nil
}

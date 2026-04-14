package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"velarix/core"
	"velarix/store"
)

func (s *Store) Append(entry store.JournalEntry) error {
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if entry.Timestamp == 0 {
		entry.Timestamp = s.nowMillis()
	}
	if err := s.ensureSessionRow(ctx, tx, entry.SessionID); err != nil {
		return err
	}

	var previousHead string
	if err := tx.QueryRow(ctx, `SELECT history_chain_head FROM sessions WHERE session_id = $1 FOR UPDATE`, entry.SessionID).Scan(&previousHead); err != nil {
		return err
	}

	raw, err := marshalJSON(entry)
	if err != nil {
		return err
	}
	chainHash := sha256Hex(append([]byte(previousHead), raw...))

	if _, err := tx.Exec(ctx, `
		INSERT INTO session_history (session_id, timestamp_ms, event_type, chain_hash, entry_json)
		VALUES ($1, $2, $3, $4, $5::jsonb)
	`, entry.SessionID, entry.Timestamp, string(entry.Type), chainHash, raw); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE sessions
		SET history_chain_head = $2,
		    updated_at = $3,
		    version = version + 1
		WHERE session_id = $1
	`, entry.SessionID, chainHash, entry.Timestamp); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Store) GetSessionHistory(sessionID string) ([]store.JournalEntry, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT entry_json
		FROM session_history
		WHERE session_id = $1
		ORDER BY event_id ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return decodeDocRows[store.JournalEntry](rows)
}

func (s *Store) GetSessionHistoryAfter(sessionID string, afterTimestamp int64) ([]store.JournalEntry, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT entry_json
		FROM session_history
		WHERE session_id = $1 AND timestamp_ms > $2
		ORDER BY event_id ASC
	`, sessionID, afterTimestamp)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return decodeDocRows[store.JournalEntry](rows)
}

func (s *Store) GetSessionHistoryBefore(sessionID string, beforeTimestamp int64) ([]store.JournalEntry, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT entry_json
		FROM session_history
		WHERE session_id = $1 AND timestamp_ms <= $2
		ORDER BY event_id ASC
	`, sessionID, beforeTimestamp)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return decodeDocRows[store.JournalEntry](rows)
}

func (s *Store) GetSessionHistoryChainHead(sessionID string) (string, error) {
	var head string
	err := s.pool.QueryRow(context.Background(), `SELECT history_chain_head FROM sessions WHERE session_id = $1`, sessionID).Scan(&head)
	if err != nil {
		return "", err
	}
	return head, nil
}

func (s *Store) GetSessionHistoryPage(sessionID string, cursor string, limit int, fromMs int64, toMs int64, typ string, q string) ([]store.JournalEntry, string, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	offset := parseOffsetCursor(cursor)
	q = strings.TrimSpace(strings.ToLower(q))
	typ = strings.TrimSpace(typ)

	clauses := []string{"session_id = $1"}
	args := []interface{}{sessionID}
	argPos := 2
	if fromMs != 0 {
		clauses = append(clauses, fmt.Sprintf("timestamp_ms >= $%d", argPos))
		args = append(args, fromMs)
		argPos++
	}
	if toMs != 0 {
		clauses = append(clauses, fmt.Sprintf("timestamp_ms <= $%d", argPos))
		args = append(args, toMs)
		argPos++
	}
	if typ != "" {
		clauses = append(clauses, fmt.Sprintf("event_type = $%d", argPos))
		args = append(args, typ)
		argPos++
	}
	if q != "" {
		clauses = append(clauses, fmt.Sprintf("entry_json::text ILIKE $%d", argPos))
		args = append(args, "%"+q+"%")
		argPos++
	}
	query := fmt.Sprintf(`
		SELECT entry_json
		FROM session_history
		WHERE %s
		ORDER BY event_id DESC
		LIMIT $%d OFFSET $%d
	`, strings.Join(clauses, " AND "), argPos, argPos+1)
	args = append(args, limit+1, offset)
	rows, err := s.pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	items := []store.JournalEntry{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, "", err
		}
		var entry store.JournalEntry
		if err := decodeJSON(raw, &entry); err != nil {
			continue
		}
		items = append(items, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		next = fmt.Sprintf("o:%d", offset+limit)
	}
	return items, next, nil
}

func (s *Store) SaveConfig(sessionID string, config interface{}) error {
	raw, err := marshalJSON(config)
	if err != nil {
		return err
	}
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.ensureSessionRow(ctx, tx, sessionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO session_configs (session_id, updated_at, config)
		VALUES ($1, $2, $3::jsonb)
		ON CONFLICT (session_id)
		DO UPDATE SET updated_at = EXCLUDED.updated_at, config = EXCLUDED.config
	`, sessionID, s.nowMillis(), raw); err != nil {
		return err
	}
	if err := s.bumpSessionVersion(ctx, tx, sessionID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) GetConfig(sessionID string) (*store.SessionConfig, error) {
	var raw []byte
	err := s.pool.QueryRow(context.Background(), `SELECT config FROM session_configs WHERE session_id = $1`, sessionID).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var config store.SessionConfig
	if err := decodeJSON(raw, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (s *Store) SaveSnapshot(sessionID string, snap *core.Snapshot) error {
	raw, err := marshalJSON(snap)
	if err != nil {
		return err
	}
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.ensureSessionRow(ctx, tx, sessionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO session_snapshots (session_id, timestamp_ms, snapshot)
		VALUES ($1, $2, $3::jsonb)
		ON CONFLICT (session_id)
		DO UPDATE SET timestamp_ms = EXCLUDED.timestamp_ms, snapshot = EXCLUDED.snapshot
	`, sessionID, snap.Timestamp, raw); err != nil {
		return err
	}
	if err := s.bumpSessionVersion(ctx, tx, sessionID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) GetLatestSnapshot(sessionID string) (*core.Snapshot, error) {
	var raw []byte
	err := s.pool.QueryRow(context.Background(), `SELECT snapshot FROM session_snapshots WHERE session_id = $1`, sessionID).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var snap core.Snapshot
	if err := decodeJSON(raw, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

func (s *Store) SaveExplanation(sessionID string, content json.RawMessage) (*store.ExplanationRecord, error) {
	now := s.nowMillis()
	record := &store.ExplanationRecord{
		SessionID:   sessionID,
		Timestamp:   now,
		Content:     content,
		ContentHash: sha256Hex(content),
		Tampered:    false,
	}
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := s.ensureSessionRow(ctx, tx, sessionID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO explanations (session_id, timestamp_ms, content_hash, content)
		VALUES ($1, $2, $3, $4::jsonb)
	`, sessionID, record.Timestamp, record.ContentHash, []byte(record.Content)); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return record, nil
}

func (s *Store) GetSessionExplanations(sessionID string) ([]store.ExplanationRecord, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT timestamp_ms, content_hash, content
		FROM explanations
		WHERE session_id = $1
		ORDER BY timestamp_ms DESC, id DESC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []store.ExplanationRecord{}
	for rows.Next() {
		var rec store.ExplanationRecord
		if err := rows.Scan(&rec.Timestamp, &rec.ContentHash, &rec.Content); err != nil {
			return nil, err
		}
		rec.SessionID = sessionID
		rec.Tampered = sha256Hex(rec.Content) != rec.ContentHash
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *Store) GetIdempotency(orgID string, keyHash string, maxAge time.Duration) (*store.IdempotencyRecord, error) {
	var raw []byte
	err := s.pool.QueryRow(context.Background(), `
		SELECT doc
		FROM idempotency_records
		WHERE org_id = $1 AND key_hash = $2
	`, orgID, keyHash).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var rec store.IdempotencyRecord
	if err := decodeJSON(raw, &rec); err != nil {
		return nil, err
	}
	if maxAge > 0 && rec.CreatedAt > 0 && time.Since(time.UnixMilli(rec.CreatedAt)) > maxAge {
		_, _ = s.pool.Exec(context.Background(), `DELETE FROM idempotency_records WHERE org_id = $1 AND key_hash = $2`, orgID, keyHash)
		return nil, pgx.ErrNoRows
	}
	return &rec, nil
}

func (s *Store) SaveIdempotency(orgID string, keyHash string, rec *store.IdempotencyRecord) error {
	if rec == nil {
		return fmt.Errorf("idempotency record is required")
	}
	if rec.CreatedAt == 0 {
		rec.CreatedAt = s.nowMillis()
	}
	raw, err := marshalJSON(rec)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		INSERT INTO idempotency_records (org_id, key_hash, created_at, doc)
		VALUES ($1, $2, $3, $4::jsonb)
		ON CONFLICT (org_id, key_hash)
		DO UPDATE SET created_at = EXCLUDED.created_at, doc = EXCLUDED.doc
	`, orgID, keyHash, rec.CreatedAt, raw)
	return err
}

func (s *Store) GetRateLimit(apiKey string) ([]time.Time, error) {
	var raw []byte
	err := s.pool.QueryRow(context.Background(), `SELECT doc FROM rate_limits WHERE api_key = $1`, apiKey).Scan(&raw)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var limits []time.Time
	if err := decodeJSON(raw, &limits); err != nil {
		return nil, err
	}
	return limits, nil
}

func (s *Store) SaveRateLimit(apiKey string, limits []time.Time) error {
	raw, err := marshalJSON(limits)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		INSERT INTO rate_limits (api_key, updated_at, doc)
		VALUES ($1, $2, $3::jsonb)
		ON CONFLICT (api_key)
		DO UPDATE SET updated_at = EXCLUDED.updated_at, doc = EXCLUDED.doc
	`, apiKey, s.nowMillis(), raw)
	return err
}

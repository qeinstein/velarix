package postgres

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"velarix/core"
	"velarix/store"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Store struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

type backupEnvelope struct {
	Backend     string                     `json:"backend"`
	Version     string                     `json:"version"`
	GeneratedAt int64                      `json:"generated_at"`
	Tables      map[string]json.RawMessage `json:"tables"`
}

var backupTableOrder = []string{
	"organizations",
	"users",
	"api_key_owners",
	"api_keys_legacy",
	"sessions",
	"session_configs",
	"session_history",
	"session_snapshots",
	"explanations",
	"org_metrics",
	"org_metric_timeseries",
	"org_request_breakdown",
	"notifications",
	"org_activity",
	"access_logs",
	"integrations",
	"billing_subscriptions",
	"invitations",
	"support_tickets",
	"policies",
	"export_jobs",
	"decisions",
	"decision_dependencies",
	"decision_checks",
	"search_documents",
	"idempotency_records",
	"rate_limits",
}

func Open(ctx context.Context, dsn string) (*Store, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, fmt.Errorf("postgres dsn is required")
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	s := &Store{pool: pool, now: time.Now}
	if err := s.runMigrations(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.pool == nil {
		return nil
	}
	s.pool.Close()
	return nil
}

func (s *Store) BackendName() string {
	return "postgres"
}

func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("postgres store is not initialized")
	}
	return s.pool.Ping(ctx)
}

func (s *Store) runMigrations(ctx context.Context) error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	if _, err := s.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		version := entry.Name()
		var applied bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		body, err := migrationsFS.ReadFile("migrations/" + version)
		if err != nil {
			return err
		}
		tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", version, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) nowMillis() int64 {
	if s != nil && s.now != nil {
		return s.now().UnixMilli()
	}
	return time.Now().UnixMilli()
}

func parseOffsetCursor(cursor string) int {
	if !strings.HasPrefix(cursor, "o:") {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimPrefix(cursor, "o:"))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func nextOffsetCursor(offset, limit, total int) string {
	if offset+limit >= total {
		return ""
	}
	return fmt.Sprintf("o:%d", offset+limit)
}

func marshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func decodeJSON(raw []byte, dest interface{}) error {
	if len(raw) == 0 {
		return fmt.Errorf("empty json")
	}
	return json.Unmarshal(raw, dest)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func invitationTokenHash(token string) string {
	return sha256Hex([]byte(strings.TrimSpace(token)))
}

func (s *Store) ensureSessionRow(ctx context.Context, tx pgx.Tx, sessionID string) error {
	now := s.nowMillis()
	_, err := tx.Exec(ctx, `
		INSERT INTO sessions (session_id, created_at, updated_at, last_activity_at)
		VALUES ($1, $2, $2, $2)
		ON CONFLICT (session_id) DO NOTHING
	`, sessionID, now)
	return err
}

func (s *Store) bumpSessionVersion(ctx context.Context, tx pgx.Tx, sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	if err := s.ensureSessionRow(ctx, tx, sessionID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		UPDATE sessions
		SET version = version + 1,
		    updated_at = $2
		WHERE session_id = $1
	`, sessionID, s.nowMillis())
	return err
}

func (s *Store) GetSessionVersion(sessionID string) (int64, error) {
	var version int64
	err := s.pool.QueryRow(context.Background(), `SELECT COALESCE(version, 0) FROM sessions WHERE session_id = $1`, sessionID).Scan(&version)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	return version, err
}

func (s *Store) SetSessionOrganization(sessionID, orgID string) error {
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	now := s.nowMillis()
	var existing string
	err = tx.QueryRow(ctx, `SELECT COALESCE(org_id, '') FROM sessions WHERE session_id = $1 FOR UPDATE`, sessionID).Scan(&existing)
	if err == pgx.ErrNoRows {
		_, err = tx.Exec(ctx, `
			INSERT INTO sessions (session_id, org_id, created_at, updated_at, last_activity_at)
			VALUES ($1, $2, $3, $3, $3)
		`, sessionID, orgID, now)
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	if existing != "" && existing != orgID {
		return fmt.Errorf("session belongs to a different organization")
	}
	_, err = tx.Exec(ctx, `
		UPDATE sessions
		SET org_id = $2,
		    updated_at = $3,
		    last_activity_at = GREATEST(last_activity_at, $3)
		WHERE session_id = $1
	`, sessionID, orgID, now)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) GetSessionOrganization(sessionID string) (string, error) {
	var orgID string
	err := s.pool.QueryRow(context.Background(), `SELECT COALESCE(org_id, '') FROM sessions WHERE session_id = $1`, sessionID).Scan(&orgID)
	if err != nil {
		return "", err
	}
	if orgID == "" {
		return "", fmt.Errorf("session organization not set")
	}
	return orgID, nil
}

func (s *Store) UpsertOrgSessionIndex(orgID, sessionID string, createdAt int64) error {
	if createdAt == 0 {
		createdAt = s.nowMillis()
	}
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO sessions (session_id, org_id, created_at, updated_at, last_activity_at)
		VALUES ($1, $2, $3, $3, $3)
		ON CONFLICT (session_id)
		DO UPDATE SET
		    org_id = COALESCE(sessions.org_id, EXCLUDED.org_id),
		    created_at = LEAST(sessions.created_at, EXCLUDED.created_at),
		    updated_at = GREATEST(sessions.updated_at, EXCLUDED.updated_at),
		    last_activity_at = GREATEST(sessions.last_activity_at, EXCLUDED.last_activity_at)
	`, sessionID, orgID, createdAt)
	return err
}

func (s *Store) TouchOrgSession(orgID, sessionID string, factDelta int) error {
	now := s.nowMillis()
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO sessions (session_id, org_id, created_at, updated_at, last_activity_at, fact_count)
		VALUES ($1, $2, $3, $3, $3, GREATEST($4, 0))
		ON CONFLICT (session_id)
		DO UPDATE SET
		    org_id = COALESCE(sessions.org_id, EXCLUDED.org_id),
		    updated_at = EXCLUDED.updated_at,
		    last_activity_at = EXCLUDED.last_activity_at,
		    fact_count = GREATEST(sessions.fact_count + $4, 0)
	`, sessionID, orgID, now, factDelta)
	return err
}

func (s *Store) ListOrgSessions(orgID string, cursor string, limit int) ([]store.OrgSessionMeta, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := parseOffsetCursor(cursor)
	rows, err := s.pool.Query(context.Background(), `
		SELECT session_id, created_at, last_activity_at, fact_count
		FROM sessions
		WHERE org_id = $1
		ORDER BY last_activity_at DESC, session_id DESC
		LIMIT $2 OFFSET $3
	`, orgID, limit+1, offset)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	items := []store.OrgSessionMeta{}
	for rows.Next() {
		var item store.OrgSessionMeta
		if err := rows.Scan(&item.ID, &item.CreatedAt, &item.LastActivityAt, &item.FactCount); err != nil {
			return nil, "", err
		}
		items = append(items, item)
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

func (s *Store) Backup(w io.Writer) (uint64, error) {
	if s == nil || s.pool == nil {
		return 0, fmt.Errorf("postgres store is not initialized")
	}
	ctx := context.Background()
	env := backupEnvelope{
		Backend:     "postgres",
		Version:     "v1",
		GeneratedAt: s.nowMillis(),
		Tables:      make(map[string]json.RawMessage, len(backupTableOrder)),
	}
	for _, table := range backupTableOrder {
		query := fmt.Sprintf(`SELECT COALESCE(jsonb_agg(to_jsonb(t)), '[]'::jsonb) FROM %s t`, table)
		var raw []byte
		if err := s.pool.QueryRow(ctx, query).Scan(&raw); err != nil {
			return 0, err
		}
		env.Tables[table] = append(json.RawMessage(nil), raw...)
	}
	payload, err := json.Marshal(env)
	if err != nil {
		return 0, err
	}
	n, err := w.Write(payload)
	return uint64(n), err
}

func (s *Store) Restore(r io.Reader) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("postgres store is not initialized")
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	var env backupEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return err
	}
	if env.Backend != "" && env.Backend != "postgres" {
		return fmt.Errorf("unsupported backup backend: %s", env.Backend)
	}

	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	truncateOrder := append([]string(nil), backupTableOrder...)
	for i, j := 0, len(truncateOrder)-1; i < j; i, j = i+1, j-1 {
		truncateOrder[i], truncateOrder[j] = truncateOrder[j], truncateOrder[i]
	}
	for _, table := range truncateOrder {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`TRUNCATE TABLE %s RESTART IDENTITY CASCADE`, table)); err != nil {
			return err
		}
	}

	for _, table := range backupTableOrder {
		rows := env.Tables[table]
		if len(rows) == 0 || string(rows) == "null" || string(rows) == "[]" {
			continue
		}
		query := fmt.Sprintf(`INSERT INTO %s SELECT * FROM jsonb_populate_recordset(NULL::%s, $1::jsonb)`, table, table)
		if _, err := tx.Exec(ctx, query, rows); err != nil {
			return fmt.Errorf("restore table %s: %w", table, err)
		}
	}

	return tx.Commit(ctx)
}

func (s *Store) ReplayAll(engines map[string]*core.Engine, configs map[string][]byte) error {
	return nil
}

func (s *Store) StartGC() {}

func getenvInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

var _ store.RuntimeStore = (*Store)(nil)

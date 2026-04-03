package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"velarix/store"
)

func (s *Store) GetOrganization(id string) (*store.Organization, error) {
	var raw []byte
	err := s.pool.QueryRow(context.Background(), `SELECT doc FROM organizations WHERE id = $1`, id).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var org store.Organization
	if err := decodeJSON(raw, &org); err != nil {
		return nil, err
	}
	return &org, nil
}

func (s *Store) SaveOrganization(org *store.Organization) error {
	if org == nil {
		return fmt.Errorf("organization is required")
	}
	if org.CreatedAt == 0 {
		org.CreatedAt = s.nowMillis()
	}
	if org.Settings == nil {
		org.Settings = map[string]interface{}{}
	}
	raw, err := marshalJSON(org)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		INSERT INTO organizations (id, name, created_at, is_suspended, settings, doc)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb)
		ON CONFLICT (id)
		DO UPDATE SET
		    name = EXCLUDED.name,
		    is_suspended = EXCLUDED.is_suspended,
		    settings = EXCLUDED.settings,
		    doc = EXCLUDED.doc
	`, org.ID, org.Name, org.CreatedAt, org.IsSuspended, mustJSON(org.Settings), raw)
	return err
}

func (s *Store) IncrementOrgMetric(orgID string, metric string, delta uint64) error {
	now := s.nowMillis()
	minuteStart := (now / 60000) * 60000
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO org_metrics (org_id, metric, value)
		VALUES ($1, $2, $3)
		ON CONFLICT (org_id, metric)
		DO UPDATE SET value = org_metrics.value + EXCLUDED.value
	`, orgID, metric, int64(delta))
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		INSERT INTO org_metric_timeseries (org_id, metric, bucket_ms, value)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (org_id, metric, bucket_ms)
		DO UPDATE SET value = org_metric_timeseries.value + EXCLUDED.value
	`, orgID, metric, minuteStart, int64(delta))
	return err
}

func (s *Store) GetOrgUsage(orgID string) (map[string]uint64, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT metric, value
		FROM org_metrics
		WHERE org_id = $1
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]uint64{}
	for rows.Next() {
		var metric string
		var value int64
		if err := rows.Scan(&metric, &value); err != nil {
			return nil, err
		}
		out[metric] = uint64(value)
	}
	return out, rows.Err()
}

func (s *Store) GetOrgMetricTimeseries(orgID string, metric string, fromMs int64, toMs int64, bucketMs int64) ([]store.MetricPoint, error) {
	if bucketMs <= 0 {
		bucketMs = 60000
	}
	rows, err := s.pool.Query(context.Background(), `
		SELECT bucket_ms, value
		FROM org_metric_timeseries
		WHERE org_id = $1 AND metric = $2 AND bucket_ms >= $3 AND bucket_ms <= $4
		ORDER BY bucket_ms ASC
	`, orgID, metric, (fromMs/60000)*60000, (toMs/60000)*60000)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	agg := map[int64]uint64{}
	for rows.Next() {
		var bucket int64
		var value int64
		if err := rows.Scan(&bucket, &value); err != nil {
			return nil, err
		}
		dst := (bucket / bucketMs) * bucketMs
		agg[dst] += uint64(value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	points := make([]store.MetricPoint, 0, len(agg))
	for bucket, value := range agg {
		points = append(points, store.MetricPoint{TimestampMs: bucket, Value: value})
	}
	sort.Slice(points, func(i, j int) bool { return points[i].TimestampMs < points[j].TimestampMs })
	return points, nil
}

func (s *Store) IncOrgRequestBreakdown(orgID string, endpoint string, status int, delta uint64) error {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO org_request_breakdown (org_id, endpoint, status, value)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (org_id, endpoint, status)
		DO UPDATE SET value = org_request_breakdown.value + EXCLUDED.value
	`, orgID, endpoint, status, int64(delta))
	return err
}

func (s *Store) GetOrgUsageBreakdown(orgID string) (*store.UsageBreakdown, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT endpoint, status, value
		FROM org_request_breakdown
		WHERE org_id = $1
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byEndpoint := map[string]uint64{}
	byStatus := map[string]uint64{}
	raw := map[string]map[string]uint64{}
	for rows.Next() {
		var endpoint string
		var status int
		var value int64
		if err := rows.Scan(&endpoint, &status, &value); err != nil {
			return nil, err
		}
		statusKey := fmt.Sprintf("%d", status)
		byEndpoint[endpoint] += uint64(value)
		byStatus[statusKey] += uint64(value)
		if raw[endpoint] == nil {
			raw[endpoint] = map[string]uint64{}
		}
		raw[endpoint][statusKey] += uint64(value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &store.UsageBreakdown{ByEndpoint: byEndpoint, ByStatus: byStatus, Raw: raw}, nil
}

func (s *Store) ListOrgUsers(orgID string) ([]string, error) {
	rows, err := s.pool.Query(context.Background(), `SELECT email FROM users WHERE org_id = $1 ORDER BY email ASC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		out = append(out, email)
	}
	return out, rows.Err()
}

func (s *Store) SaveNotification(orgID string, n *store.Notification) error {
	if n == nil {
		return fmt.Errorf("notification is required")
	}
	raw, err := marshalJSON(n)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		INSERT INTO notifications (org_id, notification_id, created_at, read_at, doc)
		VALUES ($1, $2, $3, $4, $5::jsonb)
		ON CONFLICT (org_id, notification_id)
		DO UPDATE SET created_at = EXCLUDED.created_at, read_at = EXCLUDED.read_at, doc = EXCLUDED.doc
	`, orgID, n.ID, n.CreatedAt, n.ReadAt, raw)
	return err
}

func (s *Store) ListNotifications(orgID string, cursor string, limit int) ([]store.Notification, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(context.Background(), `
		SELECT doc
		FROM notifications
		WHERE org_id = $1
		ORDER BY created_at DESC, notification_id DESC
	`, orgID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	items := []store.Notification{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, "", err
		}
		var item store.Notification
		if err := decodeJSON(raw, &item); err != nil {
			continue
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	offset := parseOffsetCursor(cursor)
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], nextOffsetCursor(offset, limit, len(items)), nil
}

func (s *Store) MarkNotificationRead(orgID string, notificationID string) error {
	var raw []byte
	err := s.pool.QueryRow(context.Background(), `SELECT doc FROM notifications WHERE org_id = $1 AND notification_id = $2`, orgID, notificationID).Scan(&raw)
	if err != nil {
		return err
	}
	var item store.Notification
	if err := decodeJSON(raw, &item); err != nil {
		return err
	}
	item.ReadAt = s.nowMillis()
	nextRaw, err := marshalJSON(item)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		UPDATE notifications
		SET read_at = $3, doc = $4::jsonb
		WHERE org_id = $1 AND notification_id = $2
	`, orgID, notificationID, item.ReadAt, nextRaw)
	return err
}

func (s *Store) AppendOrgActivity(orgID string, entry store.JournalEntry) error {
	raw, err := marshalJSON(entry)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		INSERT INTO org_activity (org_id, created_at, entry_json)
		VALUES ($1, $2, $3::jsonb)
	`, orgID, entry.Timestamp, raw)
	return err
}

func (s *Store) ListOrgActivityPage(orgID string, cursor string, limit int) ([]store.JournalEntry, string, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(context.Background(), `
		SELECT entry_json
		FROM org_activity
		WHERE org_id = $1
		ORDER BY created_at DESC, id DESC
	`, orgID)
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
		var item store.JournalEntry
		if err := decodeJSON(raw, &item); err != nil {
			continue
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	offset := parseOffsetCursor(cursor)
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], nextOffsetCursor(offset, limit, len(items)), nil
}

func (s *Store) AppendAccessLog(orgID string, e store.AccessLogEntry) error {
	raw, err := marshalJSON(e)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		INSERT INTO access_logs (id, org_id, created_at, entry_json)
		VALUES ($1, $2, $3, $4::jsonb)
		ON CONFLICT (id) DO UPDATE SET entry_json = EXCLUDED.entry_json, created_at = EXCLUDED.created_at, org_id = EXCLUDED.org_id
	`, e.ID, orgID, e.CreatedAt, raw)
	return err
}

func (s *Store) ListAccessLogsPage(orgID string, cursor string, limit int) ([]store.AccessLogEntry, string, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(context.Background(), `
		SELECT entry_json
		FROM access_logs
		WHERE org_id = $1
		ORDER BY created_at DESC, id DESC
	`, orgID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	items := []store.AccessLogEntry{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, "", err
		}
		var item store.AccessLogEntry
		if err := decodeJSON(raw, &item); err != nil {
			continue
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	offset := parseOffsetCursor(cursor)
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], nextOffsetCursor(offset, limit, len(items)), nil
}

func (s *Store) GetUser(email string) (*store.User, error) {
	var raw []byte
	err := s.pool.QueryRow(context.Background(), `SELECT doc FROM users WHERE email = $1`, email).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var user store.User
	if err := decodeJSON(raw, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) SaveUser(user *store.User) error {
	if user == nil {
		return fmt.Errorf("user is required")
	}
	if user.Role == "" {
		user.Role = "member"
	}
	raw, err := marshalJSON(user)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		INSERT INTO users (email, org_id, role, doc)
		VALUES ($1, $2, $3, $4::jsonb)
		ON CONFLICT (email)
		DO UPDATE SET org_id = EXCLUDED.org_id, role = EXCLUDED.role, doc = EXCLUDED.doc
	`, user.Email, user.OrgID, user.Role, raw)
	return err
}

func (s *Store) GetAPIKeyOwner(key string) ([]byte, error) {
	var email string
	err := s.pool.QueryRow(context.Background(), `SELECT email FROM api_keys_legacy WHERE api_key = $1`, key).Scan(&email)
	if err != nil {
		return nil, err
	}
	return []byte(email), nil
}

func (s *Store) GetAPIKeyOwnerByHash(keyHash string) ([]byte, error) {
	var email string
	err := s.pool.QueryRow(context.Background(), `SELECT email FROM api_key_owners WHERE key_hash = $1`, keyHash).Scan(&email)
	if err != nil {
		return nil, err
	}
	return []byte(email), nil
}

func (s *Store) SaveAPIKeyHash(keyHash, email string) error {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO api_key_owners (key_hash, email)
		VALUES ($1, $2)
		ON CONFLICT (key_hash) DO UPDATE SET email = EXCLUDED.email
	`, keyHash, email)
	return err
}

func (s *Store) DeleteAPIKeyHash(keyHash string) error {
	_, err := s.pool.Exec(context.Background(), `DELETE FROM api_key_owners WHERE key_hash = $1`, keyHash)
	return err
}

func (s *Store) DeleteAPIKey(key string) error {
	_, err := s.pool.Exec(context.Background(), `DELETE FROM api_keys_legacy WHERE api_key = $1`, key)
	return err
}

func (s *Store) SaveIntegration(orgID string, integ *store.Integration) error {
	return s.saveOrgScopedDocument("integrations", "integration_id", orgID, integ.ID, integ.UpdatedAt, integ)
}

func (s *Store) GetIntegration(orgID, id string) (*store.Integration, error) {
	var item store.Integration
	if err := s.getOrgScopedDocument("integrations", "integration_id", orgID, id, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) DeleteIntegration(orgID, id string) error {
	return s.deleteOrgScopedDocument("integrations", "integration_id", orgID, id)
}

func (s *Store) ListIntegrations(orgID string) ([]store.Integration, error) {
	items := []store.Integration{}
	if err := s.listOrgScopedDocuments("integrations", orgID, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) SaveInvitation(orgID string, inv *store.Invitation) error {
	if inv == nil {
		return fmt.Errorf("invitation is required")
	}
	raw, err := marshalJSON(inv)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		INSERT INTO invitations (org_id, invitation_id, email, token_hash, created_at, expires_at, accepted_at, revoked_at, doc)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)
		ON CONFLICT (org_id, invitation_id)
		DO UPDATE SET
		    email = EXCLUDED.email,
		    token_hash = EXCLUDED.token_hash,
		    created_at = EXCLUDED.created_at,
		    expires_at = EXCLUDED.expires_at,
		    accepted_at = EXCLUDED.accepted_at,
		    revoked_at = EXCLUDED.revoked_at,
		    doc = EXCLUDED.doc
	`, orgID, inv.ID, inv.Email, invitationTokenHash(inv.Token), inv.CreatedAt, inv.ExpiresAt, inv.AcceptedAt, inv.RevokedAt, raw)
	return err
}

func (s *Store) ListInvitations(orgID string) ([]store.Invitation, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT doc
		FROM invitations
		WHERE org_id = $1
		ORDER BY created_at DESC, invitation_id DESC
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []store.Invitation{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item store.Invitation
		if err := decodeJSON(raw, &item); err != nil {
			continue
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetInvitation(orgID, id string) (*store.Invitation, error) {
	var raw []byte
	err := s.pool.QueryRow(context.Background(), `
		SELECT doc
		FROM invitations
		WHERE org_id = $1 AND invitation_id = $2
	`, orgID, id).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var item store.Invitation
	if err := decodeJSON(raw, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) GetInvitationByToken(token string) (string, *store.Invitation, error) {
	var orgID string
	var raw []byte
	err := s.pool.QueryRow(context.Background(), `
		SELECT org_id, doc
		FROM invitations
		WHERE token_hash = $1
	`, invitationTokenHash(token)).Scan(&orgID, &raw)
	if err != nil {
		return "", nil, err
	}
	var item store.Invitation
	if err := decodeJSON(raw, &item); err != nil {
		return "", nil, err
	}
	return orgID, &item, nil
}

func (s *Store) UpdateInvitation(orgID string, inv *store.Invitation) error {
	return s.SaveInvitation(orgID, inv)
}

func (s *Store) GetBilling(orgID string) (*store.BillingSubscription, error) {
	var raw []byte
	err := s.pool.QueryRow(context.Background(), `SELECT doc FROM billing_subscriptions WHERE org_id = $1`, orgID).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var item store.BillingSubscription
	if err := decodeJSON(raw, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) SaveBilling(orgID string, sub *store.BillingSubscription) error {
	if sub == nil {
		return fmt.Errorf("billing subscription is required")
	}
	raw, err := marshalJSON(sub)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		INSERT INTO billing_subscriptions (org_id, updated_at, doc)
		VALUES ($1, $2, $3::jsonb)
		ON CONFLICT (org_id)
		DO UPDATE SET updated_at = EXCLUDED.updated_at, doc = EXCLUDED.doc
	`, orgID, sub.UpdatedAt, raw)
	return err
}

func (s *Store) SaveTicket(orgID string, t *store.SupportTicket) error {
	return s.saveOrgScopedDocument("support_tickets", "ticket_id", orgID, t.ID, t.UpdatedAt, t)
}

func (s *Store) ListTickets(orgID string) ([]store.SupportTicket, error) {
	items := []store.SupportTicket{}
	if err := s.listOrgScopedDocuments("support_tickets", orgID, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) GetTicket(orgID, id string) (*store.SupportTicket, error) {
	var item store.SupportTicket
	if err := s.getOrgScopedDocument("support_tickets", "ticket_id", orgID, id, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) SavePolicy(orgID string, p *store.Policy) error {
	return s.saveOrgScopedDocument("policies", "policy_id", orgID, p.ID, p.UpdatedAt, p)
}

func (s *Store) ListPolicies(orgID string) ([]store.Policy, error) {
	items := []store.Policy{}
	if err := s.listOrgScopedDocuments("policies", orgID, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) GetPolicy(orgID, id string) (*store.Policy, error) {
	var item store.Policy
	if err := s.getOrgScopedDocument("policies", "policy_id", orgID, id, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) DeletePolicy(orgID, id string) error {
	return s.deleteOrgScopedDocument("policies", "policy_id", orgID, id)
}

func (s *Store) SaveExportJob(job *store.ExportJob) error {
	if job == nil {
		return fmt.Errorf("export job is required")
	}
	raw, err := marshalJSON(job)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		INSERT INTO export_jobs (session_id, job_id, org_id, created_at, completed_at, status, doc)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
		ON CONFLICT (session_id, job_id)
		DO UPDATE SET
		    org_id = EXCLUDED.org_id,
		    created_at = EXCLUDED.created_at,
		    completed_at = EXCLUDED.completed_at,
		    status = EXCLUDED.status,
		    doc = EXCLUDED.doc
	`, job.SessionID, job.ID, job.OrgID, job.CreatedAt, job.CompletedAt, job.Status, raw)
	return err
}

func (s *Store) GetExportJob(sessionID string, id string) (*store.ExportJob, error) {
	var raw []byte
	err := s.pool.QueryRow(context.Background(), `
		SELECT doc
		FROM export_jobs
		WHERE session_id = $1 AND job_id = $2
	`, sessionID, id).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var job store.ExportJob
	if err := decodeJSON(raw, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *Store) SaveExportJobResult(sessionID string, id string, contentType string, filename string, data []byte, errMsg string) error {
	job, err := s.GetExportJob(sessionID, id)
	if err != nil {
		return err
	}
	job.CompletedAt = s.nowMillis()
	job.ContentType = contentType
	job.Filename = filename
	job.SizeBytes = int64(len(data))
	job.Error = errMsg
	if errMsg != "" {
		job.Status = "error"
	} else {
		job.Status = "done"
	}
	raw, err := marshalJSON(job)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		UPDATE export_jobs
		SET completed_at = $4, status = $5, data = $6, doc = $7::jsonb
		WHERE session_id = $1 AND job_id = $2
	`, sessionID, id, job.OrgID, job.CompletedAt, job.Status, data, raw)
	return err
}

func (s *Store) GetExportJobData(sessionID string, id string) ([]byte, error) {
	var data []byte
	err := s.pool.QueryRow(context.Background(), `
		SELECT COALESCE(data, ''::bytea)
		FROM export_jobs
		WHERE session_id = $1 AND job_id = $2
	`, sessionID, id).Scan(&data)
	return data, err
}

func (s *Store) ListExportJobs(sessionID string, limit int) ([]store.ExportJob, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(context.Background(), `
		SELECT doc
		FROM export_jobs
		WHERE session_id = $1
		ORDER BY created_at DESC, job_id DESC
		LIMIT $2
	`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []store.ExportJob{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item store.ExportJob
		if err := decodeJSON(raw, &item); err != nil {
			continue
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) EnforceRetention(now time.Time) (*store.RetentionReport, error) {
	rows, err := s.pool.Query(context.Background(), `SELECT doc FROM organizations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type retention struct {
		orgID        string
		activityDays int
		accessDays   int
		notifyDays   int
	}
	orgs := []retention{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var org store.Organization
		if err := decodeJSON(raw, &org); err != nil {
			continue
		}
		orgs = append(orgs, retention{
			orgID:        org.ID,
			activityDays: orgSettingAsInt(&org, "retention_days_activity", 30),
			accessDays:   orgSettingAsInt(&org, "retention_days_access_logs", 30),
			notifyDays:   orgSettingAsInt(&org, "retention_days_notifications", 30),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	report := &store.RetentionReport{}
	for _, org := range orgs {
		activityCutoff := now.Add(-time.Duration(org.activityDays) * 24 * time.Hour).UnixMilli()
		accessCutoff := now.Add(-time.Duration(org.accessDays) * 24 * time.Hour).UnixMilli()
		notifyCutoff := now.Add(-time.Duration(org.notifyDays) * 24 * time.Hour).UnixMilli()

		tag, err := s.pool.Exec(context.Background(), `DELETE FROM org_activity WHERE org_id = $1 AND created_at < $2`, org.orgID, activityCutoff)
		if err != nil {
			return nil, err
		}
		report.ActivityDeleted += int(tag.RowsAffected())

		tag, err = s.pool.Exec(context.Background(), `DELETE FROM access_logs WHERE org_id = $1 AND created_at < $2`, org.orgID, accessCutoff)
		if err != nil {
			return nil, err
		}
		report.AccessLogsDeleted += int(tag.RowsAffected())

		tag, err = s.pool.Exec(context.Background(), `DELETE FROM notifications WHERE org_id = $1 AND created_at < $2`, org.orgID, notifyCutoff)
		if err != nil {
			return nil, err
		}
		report.NotificationsDeleted += int(tag.RowsAffected())
	}
	return report, nil
}

func (s *Store) saveOrgScopedDocument(table string, idColumn string, orgID string, id string, updatedAt int64, v interface{}) error {
	raw, err := marshalJSON(v)
	if err != nil {
		return err
	}
	if updatedAt == 0 {
		updatedAt = s.nowMillis()
	}
	_, err = s.pool.Exec(context.Background(), fmt.Sprintf(`
		INSERT INTO %s (org_id, %s, updated_at, doc)
		VALUES ($1, $2, $3, $4::jsonb)
		ON CONFLICT (org_id, %s)
		DO UPDATE SET updated_at = EXCLUDED.updated_at, doc = EXCLUDED.doc
	`, table, idColumn, idColumn), orgID, id, updatedAt, raw)
	return err
}

func (s *Store) getOrgScopedDocument(table string, idColumn string, orgID string, id string, dest interface{}) error {
	var raw []byte
	err := s.pool.QueryRow(context.Background(), fmt.Sprintf(`
		SELECT doc
		FROM %s
		WHERE org_id = $1 AND %s = $2
	`, table, idColumn), orgID, id).Scan(&raw)
	if err != nil {
		return err
	}
	return decodeJSON(raw, dest)
}

func (s *Store) deleteOrgScopedDocument(table string, idColumn string, orgID string, id string) error {
	_, err := s.pool.Exec(context.Background(), fmt.Sprintf(`DELETE FROM %s WHERE org_id = $1 AND %s = $2`, table, idColumn), orgID, id)
	return err
}

func (s *Store) listOrgScopedDocuments(table string, orgID string, dest interface{}) error {
	rows, err := s.pool.Query(context.Background(), fmt.Sprintf(`
		SELECT doc
		FROM %s
		WHERE org_id = $1
		ORDER BY updated_at DESC
	`, table), orgID)
	if err != nil {
		return err
	}
	defer rows.Close()

	switch out := dest.(type) {
	case *[]store.Integration:
		for rows.Next() {
			var raw []byte
			if err := rows.Scan(&raw); err != nil {
				return err
			}
			var item store.Integration
			if err := decodeJSON(raw, &item); err == nil {
				*out = append(*out, item)
			}
		}
	case *[]store.SupportTicket:
		for rows.Next() {
			var raw []byte
			if err := rows.Scan(&raw); err != nil {
				return err
			}
			var item store.SupportTicket
			if err := decodeJSON(raw, &item); err == nil {
				*out = append(*out, item)
			}
		}
	case *[]store.Policy:
		for rows.Next() {
			var raw []byte
			if err := rows.Scan(&raw); err != nil {
				return err
			}
			var item store.Policy
			if err := decodeJSON(raw, &item); err == nil {
				*out = append(*out, item)
			}
		}
	default:
		return fmt.Errorf("unsupported destination list type")
	}
	return rows.Err()
}

func orgSettingAsInt(org *store.Organization, key string, def int) int {
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
		if n, err := v.Int64(); err == nil {
			return int(n)
		}
	}
	return def
}

func mustJSON(v interface{}) []byte {
	raw, _ := json.Marshal(v)
	return raw
}

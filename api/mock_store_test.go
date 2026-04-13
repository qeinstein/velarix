package api

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"velarix/core"
	"velarix/store"
)

type MockStore struct {
	// SessionStateStore
	AppendFunc                     func(entry store.JournalEntry) error
	GetSessionHistoryFunc          func(sessionID string) ([]store.JournalEntry, error)
	GetSessionHistoryAfterFunc     func(sessionID string, afterTimestamp int64) ([]store.JournalEntry, error)
	GetSessionHistoryBeforeFunc    func(sessionID string, beforeTimestamp int64) ([]store.JournalEntry, error)
	GetSessionHistoryChainHeadFunc func(sessionID string) (string, error)
	GetSessionHistoryPageFunc      func(sessionID string, cursor string, limit int, fromMs int64, toMs int64, typ string, q string) ([]store.JournalEntry, string, error)
	GetConfigFunc                  func(sessionID string) (*store.SessionConfig, error)
	SaveConfigFunc                 func(sessionID string, config interface{}) error
	SaveSnapshotFunc               func(sessionID string, snap *core.Snapshot) error
	GetLatestSnapshotFunc          func(sessionID string) (*core.Snapshot, error)
	SaveExplanationFunc            func(sessionID string, content json.RawMessage) (*store.ExplanationRecord, error)
	GetSessionExplanationsFunc     func(sessionID string) ([]store.ExplanationRecord, error)
	SetSessionOrganizationFunc     func(sessionID, orgID string) error
	GetSessionOrganizationFunc     func(sessionID string) (string, error)
	GetSessionVersionFunc          func(sessionID string) (int64, error)

	// OrgMetadataStore
	GetOrganizationFunc           func(id string) (*store.Organization, error)
	SaveOrganizationFunc          func(org *store.Organization) error
	UpsertOrgSessionIndexFunc     func(orgID, sessionID string, createdAt int64) error
	TouchOrgSessionFunc           func(orgID, sessionID string, factDelta int) error
	GetOrgUsageFunc               func(orgID string) (map[string]uint64, error)
	GetOrgMetricTimeseriesFunc    func(orgID string, metric string, fromMs int64, toMs int64, bucketMs int64) ([]store.MetricPoint, error)
	GetOrgUsageBreakdownFunc      func(orgID string) (*store.UsageBreakdown, error)
	IncrementOrgMetricFunc        func(orgID string, metric string, delta uint64) error
	IncOrgRequestBreakdownFunc    func(orgID string, endpoint string, status int, delta uint64) error
	ListOrgUsersFunc              func(orgID string) ([]string, error)
	SaveNotificationFunc          func(orgID string, n *store.Notification) error
	ListNotificationsFunc         func(orgID string, cursor string, limit int) ([]store.Notification, string, error)
	MarkNotificationReadFunc      func(orgID string, notificationID string) error

	// SearchStore
	ListOrgSessionsFunc      func(orgID string, cursor string, limit int) ([]store.OrgSessionMeta, string, error)
	PatchOrgSessionMetaFunc  func(orgID, sessionID, name, description string) error
	DeleteOrgSessionFunc     func(orgID, sessionID string) error
	ListOrgActivityPageFunc  func(orgID string, cursor string, limit int) ([]store.JournalEntry, string, error)
	ListAccessLogsPageFunc   func(orgID string, cursor string, limit int) ([]store.AccessLogEntry, string, error)
	UpsertSearchDocumentsFunc func(docs []store.SearchDocument) error
	SearchDocumentsFunc      func(orgID string, filter store.SearchDocumentsFilter) ([]store.SearchDocument, string, error)

	// AuditStore
	AppendOrgActivityFunc func(orgID string, entry store.JournalEntry) error
	AppendAccessLogFunc   func(orgID string, e store.AccessLogEntry) error

	// ExportJobStore
	SaveExportJobFunc       func(job *store.ExportJob) error
	GetExportJobFunc        func(sessionID string, id string) (*store.ExportJob, error)
	SaveExportJobResultFunc func(sessionID string, id string, contentType string, filename string, data []byte, errMsg string) error
	GetExportJobDataFunc    func(sessionID string, id string) ([]byte, error)
	ListExportJobsFunc      func(sessionID string, limit int) ([]store.ExportJob, error)

	// DecisionStore
	SaveDecisionFunc             func(decision *store.Decision) error
	GetDecisionFunc              func(sessionID string, decisionID string) (*store.Decision, error)
	ListSessionDecisionsFunc     func(sessionID string, filter store.DecisionListFilter) ([]store.Decision, error)
	ListOrgDecisionsFunc         func(orgID string, filter store.DecisionListFilter) ([]store.Decision, error)
	SaveDecisionDependenciesFunc func(sessionID string, decisionID string, deps []store.DecisionDependency) error
	GetDecisionDependenciesFunc  func(sessionID string, decisionID string) ([]store.DecisionDependency, error)
	SaveDecisionCheckFunc        func(sessionID string, decisionID string, check *store.DecisionCheck) error
	GetLatestDecisionCheckFunc   func(sessionID string, decisionID string) (*store.DecisionCheck, error)

	// IdentityStore
	GetUserFunc            func(email string) (*store.User, error)
	SaveUserFunc           func(user *store.User) error
	GetAPIKeyOwnerFunc     func(key string) ([]byte, error)
	GetAPIKeyOwnerByHashFunc func(keyHash string) ([]byte, error)
	SaveAPIKeyHashFunc     func(keyHash, email string) error
	DeleteAPIKeyHashFunc   func(keyHash string) error
	DeleteAPIKeyFunc       func(key string) error

	// IdempotencyStore
	GetIdempotencyFunc  func(orgID string, keyHash string, maxAge time.Duration) (*store.IdempotencyRecord, error)
	SaveIdempotencyFunc func(orgID string, keyHash string, rec *store.IdempotencyRecord) error

	// RateLimitStore
	GetRateLimitFunc  func(apiKey string) ([]time.Time, error)
	SaveRateLimitFunc func(apiKey string, limits []time.Time) error

	// IntegrationStore
	SaveIntegrationFunc   func(orgID string, integ *store.Integration) error
	GetIntegrationFunc    func(orgID, id string) (*store.Integration, error)
	DeleteIntegrationFunc func(orgID, id string) error
	ListIntegrationsFunc  func(orgID string) ([]store.Integration, error)

	// InvitationStore
	SaveInvitationFunc       func(orgID string, inv *store.Invitation) error
	ListInvitationsFunc      func(orgID string) ([]store.Invitation, error)
	GetInvitationFunc        func(orgID, id string) (*store.Invitation, error)
	GetInvitationByTokenFunc func(token string) (string, *store.Invitation, error)
	UpdateInvitationFunc     func(orgID string, inv *store.Invitation) error

	// BillingStore
	GetBillingFunc  func(orgID string) (*store.BillingSubscription, error)
	SaveBillingFunc func(orgID string, sub *store.BillingSubscription) error

	// TicketStore
	SaveTicketFunc  func(orgID string, t *store.SupportTicket) error
	ListTicketsFunc func(orgID string) ([]store.SupportTicket, error)
	GetTicketFunc   func(orgID, id string) (*store.SupportTicket, error)

	// PolicyStore
	SavePolicyFunc   func(orgID string, p *store.Policy) error
	ListPoliciesFunc func(orgID string) ([]store.Policy, error)
	GetPolicyFunc    func(orgID, id string) (*store.Policy, error)
	DeletePolicyFunc func(orgID, id string) error

	// BackupStore
	BackupFunc  func(w io.Writer) (uint64, error)
	RestoreFunc func(r io.Reader) error

	// RetentionStore
	EnforceRetentionFunc func(now time.Time) (*store.RetentionReport, error)
}

func (m *MockStore) Append(entry store.JournalEntry) error {
	if m.AppendFunc != nil { return m.AppendFunc(entry) }
	return nil
}
func (m *MockStore) GetSessionHistory(sessionID string) ([]store.JournalEntry, error) {
	if m.GetSessionHistoryFunc != nil { return m.GetSessionHistoryFunc(sessionID) }
	return nil, nil
}
func (m *MockStore) GetSessionHistoryAfter(sessionID string, afterTimestamp int64) ([]store.JournalEntry, error) {
	if m.GetSessionHistoryAfterFunc != nil { return m.GetSessionHistoryAfterFunc(sessionID, afterTimestamp) }
	return nil, nil
}
func (m *MockStore) GetSessionHistoryBefore(sessionID string, beforeTimestamp int64) ([]store.JournalEntry, error) {
	if m.GetSessionHistoryBeforeFunc != nil { return m.GetSessionHistoryBeforeFunc(sessionID, beforeTimestamp) }
	return nil, nil
}
func (m *MockStore) GetSessionHistoryChainHead(sessionID string) (string, error) {
	if m.GetSessionHistoryChainHeadFunc != nil { return m.GetSessionHistoryChainHeadFunc(sessionID) }
	return "", nil
}
func (m *MockStore) GetSessionHistoryPage(sessionID string, cursor string, limit int, fromMs int64, toMs int64, typ string, q string) ([]store.JournalEntry, string, error) {
	if m.GetSessionHistoryPageFunc != nil { return m.GetSessionHistoryPageFunc(sessionID, cursor, limit, fromMs, toMs, typ, q) }
	return nil, "", nil
}
func (m *MockStore) GetConfig(sessionID string) (*store.SessionConfig, error) {
	if m.GetConfigFunc != nil { return m.GetConfigFunc(sessionID) }
	return &store.SessionConfig{}, nil
}
func (m *MockStore) SaveConfig(sessionID string, config interface{}) error {
	if m.SaveConfigFunc != nil { return m.SaveConfigFunc(sessionID, config) }
	return nil
}
func (m *MockStore) SaveSnapshot(sessionID string, snap *core.Snapshot) error {
	if m.SaveSnapshotFunc != nil { return m.SaveSnapshotFunc(sessionID, snap) }
	return nil
}
func (m *MockStore) GetLatestSnapshot(sessionID string) (*core.Snapshot, error) {
	if m.GetLatestSnapshotFunc != nil { return m.GetLatestSnapshotFunc(sessionID) }
	return nil, nil
}
func (m *MockStore) SaveExplanation(sessionID string, content json.RawMessage) (*store.ExplanationRecord, error) {
	if m.SaveExplanationFunc != nil { return m.SaveExplanationFunc(sessionID, content) }
	return &store.ExplanationRecord{}, nil
}
func (m *MockStore) GetSessionExplanations(sessionID string) ([]store.ExplanationRecord, error) {
	if m.GetSessionExplanationsFunc != nil { return m.GetSessionExplanationsFunc(sessionID) }
	return nil, nil
}
func (m *MockStore) SetSessionOrganization(sessionID, orgID string) error {
	if m.SetSessionOrganizationFunc != nil { return m.SetSessionOrganizationFunc(sessionID, orgID) }
	return nil
}
func (m *MockStore) GetSessionOrganization(sessionID string) (string, error) {
	if m.GetSessionOrganizationFunc != nil { return m.GetSessionOrganizationFunc(sessionID) }
	return "", nil
}
func (m *MockStore) GetSessionVersion(sessionID string) (int64, error) {
	if m.GetSessionVersionFunc != nil { return m.GetSessionVersionFunc(sessionID) }
	return 0, nil
}
func (m *MockStore) GetOrganization(id string) (*store.Organization, error) {
	if m.GetOrganizationFunc != nil { return m.GetOrganizationFunc(id) }
	return &store.Organization{ID: id}, nil
}
func (m *MockStore) SaveOrganization(org *store.Organization) error {
	if m.SaveOrganizationFunc != nil { return m.SaveOrganizationFunc(org) }
	return nil
}
func (m *MockStore) UpsertOrgSessionIndex(orgID, sessionID string, createdAt int64) error {
	if m.UpsertOrgSessionIndexFunc != nil { return m.UpsertOrgSessionIndexFunc(orgID, sessionID, createdAt) }
	return nil
}
func (m *MockStore) TouchOrgSession(orgID, sessionID string, factDelta int) error {
	if m.TouchOrgSessionFunc != nil { return m.TouchOrgSessionFunc(orgID, sessionID, factDelta) }
	return nil
}
func (m *MockStore) GetOrgUsage(orgID string) (map[string]uint64, error) {
	if m.GetOrgUsageFunc != nil { return m.GetOrgUsageFunc(orgID) }
	return nil, nil
}
func (m *MockStore) GetOrgMetricTimeseries(orgID string, metric string, fromMs int64, toMs int64, bucketMs int64) ([]store.MetricPoint, error) {
	if m.GetOrgMetricTimeseriesFunc != nil { return m.GetOrgMetricTimeseriesFunc(orgID, metric, fromMs, toMs, bucketMs) }
	return nil, nil
}
func (m *MockStore) GetOrgUsageBreakdown(orgID string) (*store.UsageBreakdown, error) {
	if m.GetOrgUsageBreakdownFunc != nil { return m.GetOrgUsageBreakdownFunc(orgID) }
	return &store.UsageBreakdown{}, nil
}
func (m *MockStore) IncrementOrgMetric(orgID string, metric string, delta uint64) error {
	if m.IncrementOrgMetricFunc != nil { return m.IncrementOrgMetricFunc(orgID, metric, delta) }
	return nil
}
func (m *MockStore) IncOrgRequestBreakdown(orgID string, endpoint string, status int, delta uint64) error {
	if m.IncOrgRequestBreakdownFunc != nil { return m.IncOrgRequestBreakdownFunc(orgID, endpoint, status, delta) }
	return nil
}
func (m *MockStore) ListOrgUsers(orgID string) ([]string, error) {
	if m.ListOrgUsersFunc != nil { return m.ListOrgUsersFunc(orgID) }
	return nil, nil
}
func (m *MockStore) SaveNotification(orgID string, n *store.Notification) error {
	if m.SaveNotificationFunc != nil { return m.SaveNotificationFunc(orgID, n) }
	return nil
}
func (m *MockStore) ListNotifications(orgID string, cursor string, limit int) ([]store.Notification, string, error) {
	if m.ListNotificationsFunc != nil { return m.ListNotificationsFunc(orgID, cursor, limit) }
	return nil, "", nil
}
func (m *MockStore) MarkNotificationRead(orgID string, notificationID string) error {
	if m.MarkNotificationReadFunc != nil { return m.MarkNotificationReadFunc(orgID, notificationID) }
	return nil
}
func (m *MockStore) ListOrgSessions(orgID string, cursor string, limit int) ([]store.OrgSessionMeta, string, error) {
	if m.ListOrgSessionsFunc != nil { return m.ListOrgSessionsFunc(orgID, cursor, limit) }
	return nil, "", nil
}
func (m *MockStore) PatchOrgSessionMeta(orgID, sessionID, name, description string) error {
	if m.PatchOrgSessionMetaFunc != nil { return m.PatchOrgSessionMetaFunc(orgID, sessionID, name, description) }
	return nil
}
func (m *MockStore) DeleteOrgSession(orgID, sessionID string) error {
	if m.DeleteOrgSessionFunc != nil { return m.DeleteOrgSessionFunc(orgID, sessionID) }
	return nil
}
func (m *MockStore) ListOrgActivityPage(orgID string, cursor string, limit int) ([]store.JournalEntry, string, error) {
	if m.ListOrgActivityPageFunc != nil { return m.ListOrgActivityPageFunc(orgID, cursor, limit) }
	return nil, "", nil
}
func (m *MockStore) ListAccessLogsPage(orgID string, cursor string, limit int) ([]store.AccessLogEntry, string, error) {
	if m.ListAccessLogsPageFunc != nil { return m.ListAccessLogsPageFunc(orgID, cursor, limit) }
	return nil, "", nil
}
func (m *MockStore) UpsertSearchDocuments(docs []store.SearchDocument) error {
	if m.UpsertSearchDocumentsFunc != nil { return m.UpsertSearchDocumentsFunc(docs) }
	return nil
}
func (m *MockStore) SearchDocuments(orgID string, filter store.SearchDocumentsFilter) ([]store.SearchDocument, string, error) {
	if m.SearchDocumentsFunc != nil { return m.SearchDocumentsFunc(orgID, filter) }
	return nil, "", nil
}
func (m *MockStore) AppendOrgActivity(orgID string, entry store.JournalEntry) error {
	if m.AppendOrgActivityFunc != nil { return m.AppendOrgActivityFunc(orgID, entry) }
	return nil
}
func (m *MockStore) AppendAccessLog(orgID string, e store.AccessLogEntry) error {
	if m.AppendAccessLogFunc != nil { return m.AppendAccessLogFunc(orgID, e) }
	return nil
}
func (m *MockStore) SaveExportJob(job *store.ExportJob) error {
	if m.SaveExportJobFunc != nil { return m.SaveExportJobFunc(job) }
	return nil
}
func (m *MockStore) GetExportJob(sessionID string, id string) (*store.ExportJob, error) {
	if m.GetExportJobFunc != nil { return m.GetExportJobFunc(sessionID, id) }
	return nil, nil
}
func (m *MockStore) SaveExportJobResult(sessionID string, id string, contentType string, filename string, data []byte, errMsg string) error {
	if m.SaveExportJobResultFunc != nil { return m.SaveExportJobResultFunc(sessionID, id, contentType, filename, data, errMsg) }
	return nil
}
func (m *MockStore) GetExportJobData(sessionID string, id string) ([]byte, error) {
	if m.GetExportJobDataFunc != nil { return m.GetExportJobDataFunc(sessionID, id) }
	return nil, nil
}
func (m *MockStore) ListExportJobs(sessionID string, limit int) ([]store.ExportJob, error) {
	if m.ListExportJobsFunc != nil { return m.ListExportJobsFunc(sessionID, limit) }
	return nil, nil
}
func (m *MockStore) SaveDecision(decision *store.Decision) error {
	if m.SaveDecisionFunc != nil { return m.SaveDecisionFunc(decision) }
	return nil
}
func (m *MockStore) GetDecision(sessionID string, decisionID string) (*store.Decision, error) {
	if m.GetDecisionFunc != nil { return m.GetDecisionFunc(sessionID, decisionID) }
	return nil, nil
}
func (m *MockStore) ListSessionDecisions(sessionID string, filter store.DecisionListFilter) ([]store.Decision, error) {
	if m.ListSessionDecisionsFunc != nil { return m.ListSessionDecisionsFunc(sessionID, filter) }
	return nil, nil
}
func (m *MockStore) ListOrgDecisions(orgID string, filter store.DecisionListFilter) ([]store.Decision, error) {
	if m.ListOrgDecisionsFunc != nil { return m.ListOrgDecisionsFunc(orgID, filter) }
	return nil, nil
}
func (m *MockStore) SaveDecisionDependencies(sessionID string, decisionID string, deps []store.DecisionDependency) error {
	if m.SaveDecisionDependenciesFunc != nil { return m.SaveDecisionDependenciesFunc(sessionID, decisionID, deps) }
	return nil
}
func (m *MockStore) GetDecisionDependencies(sessionID string, decisionID string) ([]store.DecisionDependency, error) {
	if m.GetDecisionDependenciesFunc != nil { return m.GetDecisionDependenciesFunc(sessionID, decisionID) }
	return nil, nil
}
func (m *MockStore) SaveDecisionCheck(sessionID string, decisionID string, check *store.DecisionCheck) error {
	if m.SaveDecisionCheckFunc != nil { return m.SaveDecisionCheckFunc(sessionID, decisionID, check) }
	return nil
}
func (m *MockStore) GetLatestDecisionCheck(sessionID string, decisionID string) (*store.DecisionCheck, error) {
	if m.GetLatestDecisionCheckFunc != nil { return m.GetLatestDecisionCheckFunc(sessionID, decisionID) }
	return nil, nil
}
func (m *MockStore) GetUser(email string) (*store.User, error) {
	if m.GetUserFunc != nil { return m.GetUserFunc(email) }
	return nil, nil
}
func (m *MockStore) SaveUser(user *store.User) error {
	if m.SaveUserFunc != nil { return m.SaveUserFunc(user) }
	return nil
}
func (m *MockStore) GetAPIKeyOwner(key string) ([]byte, error) {
	if m.GetAPIKeyOwnerFunc != nil { return m.GetAPIKeyOwnerFunc(key) }
	return nil, nil
}
func (m *MockStore) GetAPIKeyOwnerByHash(keyHash string) ([]byte, error) {
	if m.GetAPIKeyOwnerByHashFunc != nil { return m.GetAPIKeyOwnerByHashFunc(keyHash) }
	return nil, nil
}
func (m *MockStore) SaveAPIKeyHash(keyHash, email string) error {
	if m.SaveAPIKeyHashFunc != nil { return m.SaveAPIKeyHashFunc(keyHash, email) }
	return nil
}
func (m *MockStore) DeleteAPIKeyHash(keyHash string) error {
	if m.DeleteAPIKeyHashFunc != nil { return m.DeleteAPIKeyHashFunc(keyHash) }
	return nil
}
func (m *MockStore) DeleteAPIKey(key string) error {
	if m.DeleteAPIKeyFunc != nil { return m.DeleteAPIKeyFunc(key) }
	return nil
}
func (m *MockStore) GetIdempotency(orgID string, keyHash string, maxAge time.Duration) (*store.IdempotencyRecord, error) {
	if m.GetIdempotencyFunc != nil { return m.GetIdempotencyFunc(orgID, keyHash, maxAge) }
	return nil, nil
}
func (m *MockStore) SaveIdempotency(orgID string, keyHash string, rec *store.IdempotencyRecord) error {
	if m.SaveIdempotencyFunc != nil { return m.SaveIdempotencyFunc(orgID, keyHash, rec) }
	return nil
}
func (m *MockStore) GetRateLimit(apiKey string) ([]time.Time, error) {
	if m.GetRateLimitFunc != nil { return m.GetRateLimitFunc(apiKey) }
	return nil, nil
}
func (m *MockStore) SaveRateLimit(apiKey string, limits []time.Time) error {
	if m.SaveRateLimitFunc != nil { return m.SaveRateLimitFunc(apiKey, limits) }
	return nil
}
func (m *MockStore) SaveIntegration(orgID string, integ *store.Integration) error {
	if m.SaveIntegrationFunc != nil { return m.SaveIntegrationFunc(orgID, integ) }
	return nil
}
func (m *MockStore) GetIntegration(orgID, id string) (*store.Integration, error) {
	if m.GetIntegrationFunc != nil { return m.GetIntegrationFunc(orgID, id) }
	return nil, nil
}
func (m *MockStore) DeleteIntegration(orgID, id string) error {
	if m.DeleteIntegrationFunc != nil { return m.DeleteIntegrationFunc(orgID, id) }
	return nil
}
func (m *MockStore) ListIntegrations(orgID string) ([]store.Integration, error) {
	if m.ListIntegrationsFunc != nil { return m.ListIntegrationsFunc(orgID) }
	return nil, nil
}
func (m *MockStore) SaveInvitation(orgID string, inv *store.Invitation) error {
	if m.SaveInvitationFunc != nil { return m.SaveInvitationFunc(orgID, inv) }
	return nil
}
func (m *MockStore) ListInvitations(orgID string) ([]store.Invitation, error) {
	if m.ListInvitationsFunc != nil { return m.ListInvitationsFunc(orgID) }
	return nil, nil
}
func (m *MockStore) GetInvitation(orgID, id string) (*store.Invitation, error) {
	if m.GetInvitationFunc != nil { return m.GetInvitationFunc(orgID, id) }
	return nil, nil
}
func (m *MockStore) GetInvitationByToken(token string) (string, *store.Invitation, error) {
	if m.GetInvitationByTokenFunc != nil { return m.GetInvitationByTokenFunc(token) }
	return "", nil, nil
}
func (m *MockStore) UpdateInvitation(orgID string, inv *store.Invitation) error {
	if m.UpdateInvitationFunc != nil { return m.UpdateInvitationFunc(orgID, inv) }
	return nil
}
func (m *MockStore) GetBilling(orgID string) (*store.BillingSubscription, error) {
	if m.GetBillingFunc != nil { return m.GetBillingFunc(orgID) }
	return nil, nil
}
func (m *MockStore) SaveBilling(orgID string, sub *store.BillingSubscription) error {
	if m.SaveBillingFunc != nil { return m.SaveBillingFunc(orgID, sub) }
	return nil
}
func (m *MockStore) SaveTicket(orgID string, t *store.SupportTicket) error {
	if m.SaveTicketFunc != nil { return m.SaveTicketFunc(orgID, t) }
	return nil
}
func (m *MockStore) ListTickets(orgID string) ([]store.SupportTicket, error) {
	if m.ListTicketsFunc != nil { return m.ListTicketsFunc(orgID) }
	return nil, nil
}
func (m *MockStore) GetTicket(orgID, id string) (*store.SupportTicket, error) {
	if m.GetTicketFunc != nil { return m.GetTicketFunc(orgID, id) }
	return nil, nil
}
func (m *MockStore) SavePolicy(orgID string, p *store.Policy) error {
	if m.SavePolicyFunc != nil { return m.SavePolicyFunc(orgID, p) }
	return nil
}
func (m *MockStore) ListPolicies(orgID string) ([]store.Policy, error) {
	if m.ListPoliciesFunc != nil { return m.ListPoliciesFunc(orgID) }
	return nil, nil
}
func (m *MockStore) GetPolicy(orgID, id string) (*store.Policy, error) {
	if m.GetPolicyFunc != nil { return m.GetPolicyFunc(orgID, id) }
	return nil, nil
}
func (m *MockStore) DeletePolicy(orgID, id string) error {
	if m.DeletePolicyFunc != nil { return m.DeletePolicyFunc(orgID, id) }
	return nil
}
func (m *MockStore) Backup(w io.Writer) (uint64, error) {
	if m.BackupFunc != nil { return m.BackupFunc(w) }
	return 0, nil
}
func (m *MockStore) Restore(r io.Reader) error {
	if m.RestoreFunc != nil { return m.RestoreFunc(r) }
	return nil
}
func (m *MockStore) EnforceRetention(now time.Time) (*store.RetentionReport, error) {
	if m.EnforceRetentionFunc != nil { return m.EnforceRetentionFunc(now) }
	return &store.RetentionReport{}, nil
}

// HealthReporter
func (m *MockStore) BackendName() string { return "mock" }
func (m *MockStore) Ping(ctx context.Context) error { return nil }

var _ store.ServerStore = (*MockStore)(nil)

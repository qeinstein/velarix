package store

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"velarix/core"
)

// SessionStateStore owns session-scoped facts, history, configuration, and snapshots.
type SessionStateStore interface {
	Append(entry JournalEntry) error
	GetSessionHistory(sessionID string) ([]JournalEntry, error)
	GetSessionHistoryAfter(sessionID string, afterTimestamp int64) ([]JournalEntry, error)
	GetSessionHistoryBefore(sessionID string, beforeTimestamp int64) ([]JournalEntry, error)
	GetSessionHistoryChainHead(sessionID string) (string, error)
	GetSessionHistoryPage(sessionID string, cursor string, limit int, fromMs int64, toMs int64, typ string, q string) ([]JournalEntry, string, error)
	GetConfig(sessionID string) (*SessionConfig, error)
	SaveConfig(sessionID string, config interface{}) error
	SaveSnapshot(sessionID string, snap *core.Snapshot) error
	GetLatestSnapshot(sessionID string) (*core.Snapshot, error)
	SaveExplanation(sessionID string, content json.RawMessage) (*ExplanationRecord, error)
	GetSessionExplanations(sessionID string) ([]ExplanationRecord, error)
	SetSessionOrganization(sessionID, orgID string) error
	GetSessionOrganization(sessionID string) (string, error)
	GetSessionVersion(sessionID string) (int64, error)
}

// OrgMetadataStore owns org-level metadata, analytics, and notification state.
type OrgMetadataStore interface {
	GetOrganization(id string) (*Organization, error)
	SaveOrganization(org *Organization) error
	UpsertOrgSessionIndex(orgID, sessionID string, createdAt int64) error
	TouchOrgSession(orgID, sessionID string, factDelta int) error
	GetOrgUsage(orgID string) (map[string]uint64, error)
	GetOrgMetricTimeseries(orgID string, metric string, fromMs int64, toMs int64, bucketMs int64) ([]MetricPoint, error)
	GetOrgUsageBreakdown(orgID string) (*UsageBreakdown, error)
	IncrementOrgMetric(orgID string, metric string, delta uint64) error
	IncOrgRequestBreakdown(orgID string, endpoint string, status int, delta uint64) error
	ListOrgUsers(orgID string) ([]string, error)
	SaveNotification(orgID string, n *Notification) error
	ListNotifications(orgID string, cursor string, limit int) ([]Notification, string, error)
	MarkNotificationRead(orgID string, notificationID string) error
}

// SearchStore owns indexed org/session read paths used by list/search style APIs.
type SearchStore interface {
	ListOrgSessions(orgID string, cursor string, limit int) ([]OrgSessionMeta, string, error)
	ListOrgActivityPage(orgID string, cursor string, limit int) ([]JournalEntry, string, error)
	ListAccessLogsPage(orgID string, cursor string, limit int) ([]AccessLogEntry, string, error)
	UpsertSearchDocuments(docs []SearchDocument) error
	SearchDocuments(orgID string, filter SearchDocumentsFilter) ([]SearchDocument, string, error)
}

// SemanticStore owns vector-backed fact embeddings and semantic retrieval.
type SemanticStore interface {
	UpsertFactEmbedding(orgID, sessionID string, fact *core.Fact, status core.Status) error
	SemanticSearchFacts(orgID, sessionID string, queryEmbedding []float64, limit int, validOnly bool) ([]core.SemanticMatch, error)
}

// AuditStore owns append-only audit and access-log writes.
type AuditStore interface {
	AppendOrgActivity(orgID string, entry JournalEntry) error
	AppendAccessLog(orgID string, e AccessLogEntry) error
}

// ExportJobStore owns export job metadata and payloads.
type ExportJobStore interface {
	SaveExportJob(job *ExportJob) error
	GetExportJob(sessionID string, id string) (*ExportJob, error)
	SaveExportJobResult(sessionID string, id string, contentType string, filename string, data []byte, errMsg string) error
	GetExportJobData(sessionID string, id string) ([]byte, error)
	ListExportJobs(sessionID string, limit int) ([]ExportJob, error)
}

// DecisionStore owns first-class decisions, dependency edges, and execution checks.
type DecisionStore interface {
	SaveDecision(decision *Decision) error
	GetDecision(sessionID string, decisionID string) (*Decision, error)
	ListSessionDecisions(sessionID string, filter DecisionListFilter) ([]Decision, error)
	ListOrgDecisions(orgID string, filter DecisionListFilter) ([]Decision, error)
	SaveDecisionDependencies(sessionID string, decisionID string, deps []DecisionDependency) error
	GetDecisionDependencies(sessionID string, decisionID string) ([]DecisionDependency, error)
	SaveDecisionCheck(sessionID string, decisionID string, check *DecisionCheck) error
	GetLatestDecisionCheck(sessionID string, decisionID string) (*DecisionCheck, error)
}

// IdentityStore owns users, org API-key lookups, and related auth metadata.
type IdentityStore interface {
	GetUser(email string) (*User, error)
	SaveUser(user *User) error
	GetAPIKeyOwner(key string) ([]byte, error)
	GetAPIKeyOwnerByHash(keyHash string) ([]byte, error)
	SaveAPIKeyHash(keyHash, email string) error
	DeleteAPIKeyHash(keyHash string) error
	DeleteAPIKey(key string) error
}

// IdempotencyStore owns request replay records.
type IdempotencyStore interface {
	GetIdempotency(orgID string, keyHash string, maxAge time.Duration) (*IdempotencyRecord, error)
	SaveIdempotency(orgID string, keyHash string, rec *IdempotencyRecord) error
}

// RateLimitStore owns rate-limit windows.
type RateLimitStore interface {
	GetRateLimit(apiKey string) ([]time.Time, error)
	SaveRateLimit(apiKey string, limits []time.Time) error
}

// IntegrationStore owns integration records.
type IntegrationStore interface {
	SaveIntegration(orgID string, integ *Integration) error
	GetIntegration(orgID, id string) (*Integration, error)
	DeleteIntegration(orgID, id string) error
	ListIntegrations(orgID string) ([]Integration, error)
}

// InvitationStore owns invitation lifecycle state.
type InvitationStore interface {
	SaveInvitation(orgID string, inv *Invitation) error
	ListInvitations(orgID string) ([]Invitation, error)
	GetInvitation(orgID, id string) (*Invitation, error)
	GetInvitationByToken(token string) (string, *Invitation, error)
	UpdateInvitation(orgID string, inv *Invitation) error
}

// BillingStore owns billing metadata.
type BillingStore interface {
	GetBilling(orgID string) (*BillingSubscription, error)
	SaveBilling(orgID string, sub *BillingSubscription) error
}

// TicketStore owns support-ticket metadata.
type TicketStore interface {
	SaveTicket(orgID string, t *SupportTicket) error
	ListTickets(orgID string) ([]SupportTicket, error)
	GetTicket(orgID, id string) (*SupportTicket, error)
}

// PolicyStore owns stored policy metadata.
type PolicyStore interface {
	SavePolicy(orgID string, p *Policy) error
	ListPolicies(orgID string) ([]Policy, error)
	GetPolicy(orgID, id string) (*Policy, error)
	DeletePolicy(orgID, id string) error
}

// BackupStore owns snapshot-style full-store backup/restore operations.
type BackupStore interface {
	Backup(w io.Writer) (uint64, error)
	Restore(r io.Reader) error
}

// RetentionStore owns actual deletion or archival enforcement.
type RetentionStore interface {
	EnforceRetention(now time.Time) (*RetentionReport, error)
}

// ServerStore is the full store surface required by the current API server.
type ServerStore interface {
	SessionStateStore
	OrgMetadataStore
	SearchStore
	AuditStore
	ExportJobStore
	DecisionStore
	IdentityStore
	IdempotencyStore
	RateLimitStore
	IntegrationStore
	InvitationStore
	BillingStore
	TicketStore
	PolicyStore
	BackupStore
	RetentionStore
}

// RuntimeStore extends the API-facing surface with local lifecycle hooks.
type RuntimeStore interface {
	ServerStore
	ReplayAll(engines map[string]*core.Engine, configs map[string][]byte) error
	StartGC()
	Close() error
}

type HealthReporter interface {
	BackendName() string
	Ping(ctx context.Context) error
}

var _ RuntimeStore = (*BadgerStore)(nil)

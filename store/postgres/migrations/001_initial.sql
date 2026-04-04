CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS organizations (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    is_suspended BOOLEAN NOT NULL DEFAULT FALSE,
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    doc JSONB NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
    email TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'member',
    doc JSONB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_users_org_id ON users (org_id, email);

CREATE TABLE IF NOT EXISTS api_key_owners (
    key_hash TEXT PRIMARY KEY,
    email TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS api_keys_legacy (
    api_key TEXT PRIMARY KEY,
    email TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    session_id TEXT PRIMARY KEY,
    org_id TEXT,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    last_activity_at BIGINT NOT NULL,
    fact_count INTEGER NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 0,
    history_chain_head TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_sessions_org_last_activity ON sessions (org_id, last_activity_at DESC, session_id DESC);

CREATE TABLE IF NOT EXISTS session_configs (
    session_id TEXT PRIMARY KEY REFERENCES sessions(session_id) ON DELETE CASCADE,
    updated_at BIGINT NOT NULL,
    config JSONB NOT NULL
);

CREATE TABLE IF NOT EXISTS session_history (
    event_id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(session_id) ON DELETE CASCADE,
    timestamp_ms BIGINT NOT NULL,
    event_type TEXT NOT NULL,
    chain_hash TEXT NOT NULL DEFAULT '',
    entry_json JSONB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_session_history_session_event ON session_history (session_id, event_id);
CREATE INDEX IF NOT EXISTS idx_session_history_session_ts ON session_history (session_id, timestamp_ms);

CREATE TABLE IF NOT EXISTS session_snapshots (
    session_id TEXT PRIMARY KEY REFERENCES sessions(session_id) ON DELETE CASCADE,
    timestamp_ms BIGINT NOT NULL,
    snapshot JSONB NOT NULL
);

CREATE TABLE IF NOT EXISTS explanations (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(session_id) ON DELETE CASCADE,
    timestamp_ms BIGINT NOT NULL,
    content_hash TEXT NOT NULL,
    content JSONB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_explanations_session_ts ON explanations (session_id, timestamp_ms DESC, id DESC);

CREATE TABLE IF NOT EXISTS org_metrics (
    org_id TEXT NOT NULL,
    metric TEXT NOT NULL,
    value BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (org_id, metric)
);

CREATE TABLE IF NOT EXISTS org_metric_timeseries (
    org_id TEXT NOT NULL,
    metric TEXT NOT NULL,
    bucket_ms BIGINT NOT NULL,
    value BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (org_id, metric, bucket_ms)
);

CREATE TABLE IF NOT EXISTS org_request_breakdown (
    org_id TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    status INTEGER NOT NULL,
    value BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (org_id, endpoint, status)
);

CREATE TABLE IF NOT EXISTS notifications (
    org_id TEXT NOT NULL,
    notification_id TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    read_at BIGINT NOT NULL DEFAULT 0,
    doc JSONB NOT NULL,
    PRIMARY KEY (org_id, notification_id)
);
CREATE INDEX IF NOT EXISTS idx_notifications_org_created ON notifications (org_id, created_at DESC, notification_id DESC);

CREATE TABLE IF NOT EXISTS org_activity (
    id BIGSERIAL PRIMARY KEY,
    org_id TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    entry_json JSONB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_org_activity_org_created ON org_activity (org_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS access_logs (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    entry_json JSONB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_access_logs_org_created ON access_logs (org_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS integrations (
    org_id TEXT NOT NULL,
    integration_id TEXT NOT NULL,
    updated_at BIGINT NOT NULL,
    doc JSONB NOT NULL,
    PRIMARY KEY (org_id, integration_id)
);
CREATE INDEX IF NOT EXISTS idx_integrations_org_updated ON integrations (org_id, updated_at DESC, integration_id DESC);

CREATE TABLE IF NOT EXISTS billing_subscriptions (
    org_id TEXT PRIMARY KEY,
    updated_at BIGINT NOT NULL,
    doc JSONB NOT NULL
);

CREATE TABLE IF NOT EXISTS invitations (
    org_id TEXT NOT NULL,
    invitation_id TEXT NOT NULL,
    email TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    expires_at BIGINT NOT NULL,
    accepted_at BIGINT NOT NULL DEFAULT 0,
    revoked_at BIGINT NOT NULL DEFAULT 0,
    doc JSONB NOT NULL,
    PRIMARY KEY (org_id, invitation_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_invitations_token_hash ON invitations (token_hash);
CREATE INDEX IF NOT EXISTS idx_invitations_org_created ON invitations (org_id, created_at DESC, invitation_id DESC);

CREATE TABLE IF NOT EXISTS support_tickets (
    org_id TEXT NOT NULL,
    ticket_id TEXT NOT NULL,
    updated_at BIGINT NOT NULL,
    doc JSONB NOT NULL,
    PRIMARY KEY (org_id, ticket_id)
);
CREATE INDEX IF NOT EXISTS idx_support_tickets_org_updated ON support_tickets (org_id, updated_at DESC, ticket_id DESC);

CREATE TABLE IF NOT EXISTS policies (
    org_id TEXT NOT NULL,
    policy_id TEXT NOT NULL,
    updated_at BIGINT NOT NULL,
    doc JSONB NOT NULL,
    PRIMARY KEY (org_id, policy_id)
);
CREATE INDEX IF NOT EXISTS idx_policies_org_updated ON policies (org_id, updated_at DESC, policy_id DESC);

CREATE TABLE IF NOT EXISTS export_jobs (
    session_id TEXT NOT NULL,
    job_id TEXT NOT NULL,
    org_id TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    completed_at BIGINT NOT NULL DEFAULT 0,
    status TEXT NOT NULL,
    data BYTEA,
    doc JSONB NOT NULL,
    PRIMARY KEY (session_id, job_id)
);
CREATE INDEX IF NOT EXISTS idx_export_jobs_session_created ON export_jobs (session_id, created_at DESC, job_id DESC);

CREATE TABLE IF NOT EXISTS decisions (
    session_id TEXT NOT NULL,
    decision_id TEXT NOT NULL,
    org_id TEXT NOT NULL,
    status TEXT NOT NULL,
    execution_status TEXT NOT NULL,
    subject_ref TEXT NOT NULL DEFAULT '',
    target_ref TEXT NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    doc JSONB NOT NULL,
    PRIMARY KEY (session_id, decision_id)
);
CREATE INDEX IF NOT EXISTS idx_decisions_org_created ON decisions (org_id, created_at DESC, decision_id DESC);
CREATE INDEX IF NOT EXISTS idx_decisions_org_status ON decisions (org_id, status, created_at DESC, decision_id DESC);
CREATE INDEX IF NOT EXISTS idx_decisions_session_created ON decisions (session_id, created_at DESC, decision_id DESC);

CREATE TABLE IF NOT EXISTS decision_dependencies (
    session_id TEXT NOT NULL,
    decision_id TEXT NOT NULL,
    fact_id TEXT NOT NULL,
    dependency_type TEXT NOT NULL,
    required_status TEXT NOT NULL,
    doc JSONB NOT NULL,
    PRIMARY KEY (session_id, decision_id, fact_id)
);

CREATE TABLE IF NOT EXISTS decision_checks (
    session_id TEXT NOT NULL,
    decision_id TEXT NOT NULL,
    checked_at BIGINT NOT NULL,
    doc JSONB NOT NULL,
    PRIMARY KEY (session_id, decision_id, checked_at)
);
CREATE INDEX IF NOT EXISTS idx_decision_checks_latest ON decision_checks (session_id, decision_id, checked_at DESC);

CREATE TABLE IF NOT EXISTS search_documents (
    org_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    document_type TEXT NOT NULL,
    session_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '',
    subject_ref TEXT NOT NULL DEFAULT '',
    target_ref TEXT NOT NULL DEFAULT '',
    fact_id TEXT NOT NULL DEFAULT '',
    decision_id TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    doc JSONB NOT NULL,
    PRIMARY KEY (org_id, document_id)
);
CREATE INDEX IF NOT EXISTS idx_search_documents_org_updated ON search_documents (org_id, updated_at DESC, document_id DESC);
CREATE INDEX IF NOT EXISTS idx_search_documents_org_type ON search_documents (org_id, document_type, updated_at DESC, document_id DESC);

CREATE TABLE IF NOT EXISTS idempotency_records (
    org_id TEXT NOT NULL,
    key_hash TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    doc JSONB NOT NULL,
    PRIMARY KEY (org_id, key_hash)
);

CREATE TABLE IF NOT EXISTS rate_limits (
    api_key TEXT PRIMARY KEY,
    updated_at BIGINT NOT NULL,
    doc JSONB NOT NULL
);

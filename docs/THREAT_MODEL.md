# Velarix Threat Model (Pilot-Ready)

This document is a practical threat model for running Velarix as epistemic middleware for regulated AI agents.

## Assets

- Session journals (belief assertions, invalidations, decision records)
- Exported audit packs (CSV/PDF + integrity metadata)
- API keys (service auth) and JWTs (console auth)
- Organization metadata (settings, billing, integrations, team)
- Access logs / activity feeds (sensitive metadata)

## Trust boundaries

- Public network → reverse proxy / TLS terminator → Velarix API
- Console (browser) → Velarix API (JWT)
- SDK / agents → Velarix API (API keys)
- Velarix API → BadgerDB (encrypted at rest)

## Primary threats and mitigations

### 1) Unauthorized cross-tenant reads/writes
- Mitigation: strict `OrgID` checks on every org/session handler; session→org binding persisted in storage.

### 2) Stolen API keys
- Mitigation: API keys are shown once and not stored in plaintext; keys are scoped (`read|write|export|admin`), rate-limited, revocable, and rotatable.
- Operational: restrict key distribution; prefer per-environment keys; rotate on schedule and after incidents.

### 3) Stolen JWTs / session hijack
- Mitigation: production requires `VELARIX_JWT_SECRET`; JWTs expire; console endpoints are scope-checked.
- Operational: serve console over HTTPS; consider short-lived tokens + refresh flow for enterprise.

### 4) Tampering with decision records / journal integrity
- Mitigation: session journals maintain a tamper-evident hash chain head; exports include integrity metadata for verification.

### 5) Data exfiltration via exports / logs
- Mitigation: exports require `export` scope; access logs are restricted to auditor/admin; retention settings reduce exposure window.
- Operational: implement least-privilege roles; monitor export/access-log reads.

### 6) Denial of service (ingest bursts from agents)
- Mitigation: per-org write backpressure returns `503` + `Retry-After`; idempotency prevents duplicate writes under retries.
- Operational: front with a reverse proxy that enforces request size/timeouts; scale horizontally if needed.

## Hardening defaults (recommended)

- Set `VELARIX_ENV=prod`
- Set `VELARIX_ENCRYPTION_KEY` (16/24/32 bytes)
- Set `VELARIX_JWT_SECRET`
- Set `VELARIX_ALLOWED_ORIGINS` to your console origin(s)
- Use scoped API keys; avoid `admin` scope in agents
- Configure org retention via `PATCH /v1/org/settings`


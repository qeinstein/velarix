# Threat Model

This threat model is written for Velarix as an approval-guardrail service for AI-assisted internal operations.

## Protected Assets

- approval facts and their dependency graph
- decision records and dependency snapshots
- execute-check and execute outcomes
- audit trail and access logs
- API keys and JWTs
- organization-scoped operational metadata

## Trust Boundaries

- public network to reverse proxy to Velarix API
- internal operator or service to Velarix API
- Velarix API to Postgres, Redis, or local Badger
- Velarix API to any downstream execution owner

## Primary Threats

### 1. Cross-Tenant Access

Risk:

- one org reads or writes another org's sessions or decisions

Mitigation:

- strict org binding on session and org handlers
- scoped auth on every route

### 2. Stale Decision Execution

Risk:

- an upstream fact changes but a previously generated approval still executes

Mitigation:

- dependency snapshots
- fresh `execute-check`
- blocked `execute` when dependencies are invalid

### 3. Stolen Service Credentials

Risk:

- attacker replays writes or reads approval history

Mitigation:

- scoped API keys
- revocation and rotation
- rate limiting
- org-aware auditing

### 4. Tampering With Decision History

Risk:

- approval provenance becomes untrustworthy

Mitigation:

- append-only history
- verification hashes
- persisted decision and dependency records

### 5. Denial Of Service From Bursty Writers

Risk:

- approval checks degrade under agent bursts

Mitigation:

- per-org write backpressure
- idempotency keys
- distributed coordination on the production path

### 6. Data Exposure Through Broad Platform Surfaces

Risk:

- non-core routes expand the attack surface without strengthening the approval workflow

Mitigation:

- narrow the product narrative
- reduce dependence on broad admin features
- treat non-core surfaces as secondary

## Hardening Defaults

- set `VELARIX_ENV=prod`
- set `VELARIX_JWT_SECRET`
- set `VELARIX_ALLOWED_ORIGINS`
- prefer Postgres plus Redis for production-like work
- avoid broad admin credentials in agent traffic
- require execute checks immediately before final action


# V1 Implementation Spec

This document turns the wedge choice into an execution plan.

See also:

- `V1_REAL_WORLD_ENFORCEMENT_ADDENDUM.md`

Chosen wedge:

"Decision integrity for AI-assisted internal approvals."

In plain terms:

- a system records facts used by an approval or recommendation,
- downstream decisions become stale when upstream facts change,
- stale decisions are blocked from execution,
- operators can see exactly why a decision was blocked.

This is the first build that can make the company look real.

## Product Boundary

V1 is not:

- generic agent memory,
- a healthcare platform,
- a broad compliance suite,
- or a console with many tabs.

V1 is:

- one workflow-oriented API,
- one end-to-end demo,
- one reliable SDK surface,
- one production-safe backend shape.

## V1 Success Criteria

V1 is successful only if all of the following are true:

- decisions can be recorded as first-class objects,
- a decision can be checked for execution eligibility,
- an upstream fact change can invalidate a prior decision,
- the invalidation blocks execution in the API,
- explanations include machine-readable invalidation reasons,
- the service can run on more than one API instance,
- search and decision listing do not scan all session history,
- the public demo uses only shipped SDK methods.

## Non-Goals For V1

Do not build these in this phase:

- billing,
- support ticketing,
- generic policy management UI,
- healthcare-specific flows,
- broad workflow designer UI,
- full event streaming redesign unless needed for correctness.

## Repo Touchpoints

The implementation will touch these current files directly:

- `main.go`
- `api/server.go`
- `api/console_contracts.go`
- `api/auth.go`
- `api/export_builder.go`
- `api/export_jobs.go`
- `store/badger.go`
- `store/journal.go`
- `core/engine.go`
- `core/explain.go`
- `sdks/python/velarix/client.py`
- `sdks/typescript/src/client.ts`
- `demo/agent_pivot.py`

The implementation will likely add these new areas:

- `store/interfaces.go`
- `store/postgres/`
- `store/redis/`
- `api/decision_contracts.go`
- `api/search_contracts.go`
- `api/middleware_idempotency.go`
- `api/middleware_ratelimit.go`
- `api/models/`
- `docs/openapi.yaml`
- `demo/approval_integrity.py`
- `tests/contract/`
- `tests/e2e/`

## Target Architecture

The target architecture for V1 is:

- Postgres as system of record for orgs, users, sessions, decisions, searchable fact metadata, audit events, invitations, export job metadata, and decision read models
- Redis for rate limiting, idempotency coordination, and short-lived coordination primitives
- object storage for snapshots and export artifacts
- in-memory engine cache as a rebuildable optimization only

The current Badger-backed engine remains acceptable only as:

- a local development adapter,
- a test adapter,
- or a temporary migration bridge.

## Proposed Domain Model

These are the first-class objects V1 must expose:

### Session

- `session_id`
- `org_id`
- `workflow_type`
- `status`
- `created_at`
- `updated_at`

### Fact Metadata

- `fact_id`
- `session_id`
- `kind`
- `status`
- `source_type`
- `source_ref`
- `policy_version`
- `payload_hash`
- `created_at`
- `updated_at`

### Decision

- `decision_id`
- `session_id`
- `decision_type`
- `subject_ref`
- `target_ref`
- `status`
- `execution_status`
- `recommended_action`
- `policy_version`
- `explanation_summary`
- `created_by`
- `created_at`
- `updated_at`

### Decision Dependency

- `decision_id`
- `fact_id`
- `dependency_type`
- `required_status`

### Execution Check

- `decision_id`
- `executable`
- `blocked_by`
- `reason_codes`
- `checked_at`

### Audit Event

- `event_id`
- `org_id`
- `session_id`
- `decision_id`
- `event_type`
- `actor_id`
- `payload`
- `created_at`

## Proposed API Additions

Keep the existing fact APIs. Add these workflow APIs:

### Decision Write APIs

- `POST /v1/s/{session_id}/decisions`
- `POST /v1/s/{session_id}/decisions/{decision_id}/recompute`
- `POST /v1/s/{session_id}/decisions/{decision_id}/execute-check`
- `POST /v1/s/{session_id}/decisions/{decision_id}/execute`

### Decision Read APIs

- `GET /v1/s/{session_id}/decisions`
- `GET /v1/s/{session_id}/decisions/{decision_id}`
- `GET /v1/s/{session_id}/decisions/{decision_id}/lineage`
- `GET /v1/s/{session_id}/decisions/{decision_id}/why-blocked`

### Org Read APIs

- `GET /v1/org/decisions`
- `GET /v1/org/decisions/blocked`
- `GET /v1/org/search`

`/v1/org/search` must be backed by indexed metadata. It must stop reading every session history.

## Proposed Database Tables

V1 should create these Postgres tables:

- `organizations`
- `users`
- `sessions`
- `session_configs`
- `fact_metadata`
- `decisions`
- `decision_dependencies`
- `decision_checks`
- `audit_events`
- `invitations`
- `export_jobs`
- `search_documents`

Optional if needed during migration:

- `session_snapshots`
- `session_history_index`

## Proposed Redis Keys

Use Redis for short-lived coordination only:

- `ratelimit:{org_id}:{token_hash}`
- `idempotency:{org_id}:{key_hash}`
- `session-lock:{session_id}`
- `session-cache-version:{session_id}`

## Execution Order

Implement in this order. Do not skip the sequencing.

## Milestone 0: Credibility Cleanup

This milestone removes obvious reasons to reject the repo before deeper work starts.

### Ticket V1-001: Fix Product Narrative

Scope:

- rewrite the root `README.md`
- remove unsupported claims from docs and demos
- state the product in one sentence

Files:

- `README.md`
- `DUE_DILIGENCE_FIX_PLAN.md`
- `ENGINEERING_BACKLOG.md`
- `demo/`
- `docs/`

Acceptance criteria:

- a new reader can understand the product in under 60 seconds
- no doc claims healthcare-first or broad compliance platform capability
- no doc claims billing, support, or policy enforcement unless code exists

### Ticket V1-002: Remove Broken Demo Surfaces

Scope:

- delete or rewrite `demo/agent_pivot.py`
- make one demo the canonical demo

Files:

- `demo/agent_pivot.py`
- `demo/approval_integrity.py` new

Acceptance criteria:

- `demo/agent_pivot.py` no longer calls nonexistent client methods
- there is exactly one canonical demo path documented in the README

### Ticket V1-003: Strip Fake Enterprise Language

Scope:

- remove "SOC2" language from export code and docs
- hide or de-emphasize unsupported enterprise tabs and endpoints in docs

Files:

- `api/export_builder.go`
- `README.md`
- `docs/`

Acceptance criteria:

- no user-facing string implies audited compliance posture without evidence

## Milestone 1: Storage Abstraction Before Migration

This milestone is the prerequisite for every serious backend change.

### Ticket V1-101: Introduce Store Interfaces

Scope:

- stop coupling `api.Server` to `*store.BadgerStore`
- define narrow interfaces for session state, org metadata, idempotency, rate limiting, audit, export jobs, and search

Files:

- `store/interfaces.go` new
- `api/server.go`
- `main.go`

Acceptance criteria:

- `api.Server` depends on interfaces, not `*store.BadgerStore`
- the current Badger implementation still compiles behind the new interfaces

### Ticket V1-102: Split Monolithic Server Responsibilities

Scope:

- move routing and handlers out of `api/server.go`
- separate middleware, session handlers, decision handlers, and org handlers

Files:

- `api/server.go`
- `api/session_contracts.go` new
- `api/decision_contracts.go` new
- `api/org_contracts.go` new
- `api/middleware_idempotency.go` new
- `api/middleware_ratelimit.go` new

Acceptance criteria:

- `api/server.go` becomes composition and route registration only
- rate limiting and idempotency are not embedded as ad hoc helpers in the main server file

### Ticket V1-103: Keep Badger As Local Adapter Only

Scope:

- reframe `store/badger.go` as a local adapter
- remove assumptions that it is the production authority

Files:

- `store/badger.go`
- `main.go`

Acceptance criteria:

- local dev still works with Badger
- production paths no longer require embedded Badger as authoritative state

## Milestone 2: Decision Model And Execution Gating

This is the first product-defining milestone.

### Ticket V1-201: Add Decision Domain Objects

Scope:

- define request and response models for decisions
- record decisions as first-class objects, not only generic history events

Files:

- `api/models/decision.go` new
- `api/decision_contracts.go` new
- `store/interfaces.go`
- `store/badger.go`

Acceptance criteria:

- a client can create and fetch a decision
- decisions have stable IDs and machine-readable status fields

### Ticket V1-202: Derive Decision Dependencies From Facts

Scope:

- link decisions to the fact IDs they depend on
- persist dependency edges separately from raw engine history

Files:

- `core/engine.go`
- `api/decision_contracts.go`
- `store/interfaces.go`
- `store/badger.go`

Acceptance criteria:

- a decision can list its blocking dependencies without replaying the full journal
- dependency links survive process restart

### Ticket V1-203: Add Execute Check API

Scope:

- add an execution gate endpoint
- return `executable`, `reason_codes`, and `blocked_by`

Files:

- `api/decision_contracts.go`
- `core/explain.go`
- `store/interfaces.go`

Acceptance criteria:

- execution checks return machine-readable reason codes
- a stale dependency makes `executable=false`
- the response includes enough data for an operator or calling service to act

### Ticket V1-204: Block Execution In The API

Scope:

- add a decision execution endpoint
- reject execution when dependencies are invalid

Files:

- `api/decision_contracts.go`
- `api/models/decision.go`
- `store/interfaces.go`

Acceptance criteria:

- execution attempts on stale decisions return a non-2xx response
- blocked execution writes an audit event
- successful execution records the exact check timestamp and dependency state

### Ticket V1-205: Improve Explanation Payloads

Scope:

- add decision-oriented explanation summaries
- include invalidated fact IDs, source metadata, and policy version where available

Files:

- `core/explain.go`
- `api/decision_contracts.go`

Acceptance criteria:

- the API exposes both a short summary and structured detail
- explanations are usable without reading raw graph internals

## Milestone 3: Shared-State Backend

This is the architectural fix. Without it, the product remains a single-node service.

### Ticket V1-301: Add Postgres Store Package

Scope:

- create a Postgres-backed implementation of the new store interfaces
- move orgs, sessions, decision metadata, invitations, audit events, and export job metadata out of Badger

Files:

- `store/postgres/` new
- `store/interfaces.go`
- `main.go`

Acceptance criteria:

- the service can boot with a Postgres-backed store
- core metadata no longer depends on local embedded files

### Ticket V1-302: Add SQL Migrations

Scope:

- create explicit migrations for sessions, decisions, dependencies, audit events, and search documents

Files:

- `store/postgres/migrations/` new

Acceptance criteria:

- schema can be created from a clean database
- schema changes are versioned and repeatable

### Ticket V1-303: Add Redis Backed Idempotency And Rate Limiting

Scope:

- replace Badger-backed timestamp arrays and idempotency records for production mode

Files:

- `store/redis/` new
- `api/middleware_idempotency.go`
- `api/middleware_ratelimit.go`
- `main.go`

Acceptance criteria:

- multiple API instances share the same rate limit and idempotency behavior
- retry headers are stable and documented

### Ticket V1-304: Rework Session Engine Loading

Scope:

- stop global replay at process boot
- load or rebuild engines lazily from shared storage
- persist snapshots externally

Files:

- `main.go`
- `api/server.go`
- `store/journal.go`
- `store/postgres/`

Acceptance criteria:

- process boot does not replay every known session into memory
- engine state can be reconstructed safely after restart
- two instances can serve the same session without correctness drift

### Ticket V1-305: Remove Process-Local Authority

Scope:

- keep in-memory engines as caches only
- add versioned cache invalidation or optimistic rebuild semantics

Files:

- `api/server.go`
- `store/interfaces.go`
- `store/redis/`

Acceptance criteria:

- process-local `Engines`, `Configs`, and `LastAccess` are no longer the source of truth
- stale local cache cannot silently override shared state

## Milestone 4: Search And Read Models

This milestone removes the scan-everything behavior and makes the product operationally usable.

### Ticket V1-401: Add Search Documents

Scope:

- maintain indexed search rows for sessions, facts, and decisions
- stop using `ListOrgSessions(..., 10000)` as a pseudo-search strategy

Files:

- `api/console_contracts.go`
- `store/interfaces.go`
- `store/postgres/`
- `api/search_contracts.go` new

Acceptance criteria:

- org search is index-backed
- search results are paginated
- search completeness does not degrade after 10,000 sessions

### Ticket V1-402: Add Decision Read Models

Scope:

- add list endpoints for active, blocked, and executed decisions
- support filtering by status, subject, and time range

Files:

- `api/decision_contracts.go`
- `store/postgres/`

Acceptance criteria:

- operators can list blocked decisions without scanning fact history
- decision listing latency is bounded and testable

## Milestone 5: SDKs, OpenAPI, And Demo Discipline

The public surface must become coherent.

### Ticket V1-501: Publish OpenAPI Spec

Scope:

- create a versioned OpenAPI spec for stable routes
- mark experimental routes clearly

Files:

- `docs/openapi.yaml` new
- `README.md`

Acceptance criteria:

- the documented API matches live responses for stable endpoints

### Ticket V1-502: Align Python SDK

Scope:

- keep write operations on `VelarixSession`
- add decision methods on session objects
- remove example paths that imply client-level write methods

Files:

- `sdks/python/velarix/client.py`
- `sdks/python/tests/`

Acceptance criteria:

- the Python SDK exposes decision create, fetch, list, execute-check, and execute flows
- examples do not call nonexistent methods

### Ticket V1-503: Align TypeScript SDK

Scope:

- add parity with the Python decision surface
- ensure the same session-oriented model

Files:

- `sdks/typescript/src/client.ts`
- `sdks/typescript/tests/`

Acceptance criteria:

- TypeScript SDK matches the stable public API
- TypeScript and Python SDKs have the same core feature coverage

### Ticket V1-504: Build One Canonical Demo

Scope:

- create a single workflow demo that shows:
  - facts observed,
  - decision created,
  - upstream fact changed,
  - decision blocked,
  - explanation returned

Files:

- `demo/approval_integrity.py` new
- `README.md`

Acceptance criteria:

- one documented command runs the full story from a clean checkout
- the demo uses only public SDK calls

## Milestone 6: Security And Operational Reality

This milestone removes careless behavior that would kill trust with any serious customer.

### Ticket V1-601: Fix Password Reset Flow

Scope:

- stop logging reset tokens
- disable reset in production until a real delivery path exists

Files:

- `api/auth.go`

Acceptance criteria:

- reset tokens never appear in application logs
- production behavior is explicit and safe

### Ticket V1-602: Tighten Invitation Handling

Scope:

- treat invitation tokens as secrets
- audit creation, redemption, and expiration

Files:

- `api/console_contracts.go`
- `store/interfaces.go`
- `store/postgres/`

Acceptance criteria:

- invitation lifecycle is auditable
- tokens are not handled casually in logs or broad payloads

### Ticket V1-603: Real Retention Enforcement

Scope:

- add background deletion or archival jobs
- separate retention from query filtering

Files:

- `store/postgres/`
- `api/console_contracts.go`
- `api/server.go`

Acceptance criteria:

- expired records are actually deleted or archived
- retention behavior is tested

## Milestone 7: Test And Release Gate

This is the release blocker milestone.

### Ticket V1-701: Contract Test Suite

Scope:

- add API contract tests for stable routes
- run Python and TypeScript SDKs against a live test server

Files:

- `tests/contract/` new
- `sdks/python/tests/`
- `sdks/typescript/tests/`

Acceptance criteria:

- stable routes have contract coverage
- SDKs are tested against the same real server behavior

### Ticket V1-702: Multi-Instance E2E Test

Scope:

- run two API instances against shared Postgres and Redis
- verify decision invalidation and execute-check behavior

Files:

- `tests/e2e/` new
- `Dockerfile`
- `TEST_PLAN.md`

Acceptance criteria:

- two instances can serve the same org without correctness loss
- stale decisions are blocked consistently across instances

### Ticket V1-703: Backpressure And Idempotency Tests

Scope:

- test concurrent writes
- test safe retries
- test duplicate execute attempts

Files:

- `tests/`
- `api/middleware_idempotency.go`
- `api/middleware_ratelimit.go`

Acceptance criteria:

- retries are safe
- duplicate decision writes do not create duplicates
- write bursts fail predictably rather than corrupting state

## Deletions To Make Immediately

Do these before major feature work:

- remove or rewrite `demo/agent_pivot.py`
- remove "SOC2 Compliance Export" wording
- stop pitching billing and support as product capabilities
- stop using Badger-backed rate limiting as the production plan

## Definition Of Done

V1 is done only when all of the following are true:

- the service can run on shared Postgres and Redis
- decisions are first-class records
- stale decisions can be blocked by API enforcement
- explanations are decision-oriented and structured
- search is index-backed
- one demo runs cleanly from a fresh checkout
- Python and TypeScript SDKs expose the same stable workflow
- no obvious demo or SDK mismatch remains

## Implementation Prompt For A Fresh Chat

Use this prompt in a new chat when you want implementation work to start:

```text
Work in /home/fluxx/Workspace/casualdb.

Read these files first:
- /home/fluxx/Workspace/casualdb/V1_IMPLEMENTATION_SPEC.md
- /home/fluxx/Workspace/casualdb/V1_REAL_WORLD_ENFORCEMENT_ADDENDUM.md
- /home/fluxx/Workspace/casualdb/ENGINEERING_BACKLOG.md
- /home/fluxx/Workspace/casualdb/DUE_DILIGENCE_FIX_PLAN.md

Goal:
Implement the V1 wedge: decision integrity for AI-assisted internal approvals.

Execution rules:
- Do not redesign the product scope.
- Follow V1_IMPLEMENTATION_SPEC.md sequencing.
- Start with Milestone 0 and Milestone 1 only unless you can safely complete more.
- Use apply_patch for edits.
- Do not revert unrelated user changes.
- Keep the public API coherent.
- Prefer small, testable increments.

Specific outcomes I want from this chat:
1. Fix repo credibility issues called out in Milestone 0.
2. Introduce store interfaces so api.Server no longer depends directly on *store.BadgerStore.
3. Split api/server.go enough to make future decision endpoints and middleware maintainable.
4. Keep Badger working as a local adapter.
5. Run relevant tests after each logical milestone.

When you finish:
- summarize what you changed,
- list remaining blockers for Milestone 2,
- and point to the exact files touched.
```

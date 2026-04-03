# Engineering Backlog

This is the execution companion to `DUE_DILIGENCE_FIX_PLAN.md`.

It is written as a working backlog, not a vision deck.

## Assumption For This Backlog

The first wedge is:

"Prevent stale agent decisions in internal approval and compliance workflows."

Examples:

- procurement approvals,
- access approvals,
- policy checks before action execution,
- internal audit and exception workflows.

This backlog assumes the company is no longer trying to be:

- generic agent memory,
- healthcare-first,
- a broad enterprise platform,
- or a console with many weak tabs.

## V1 Outcome

Ship a product that can do all of the following for one real workflow:

- ingest facts and derived decisions,
- invalidate downstream decisions when upstream facts change,
- show replayable provenance for why a decision existed,
- prevent execution of stale decisions,
- support multi-tenant production deployment,
- survive more than one API instance.

## Release Standard

Nothing ships unless it meets all of these:

- documented API contract,
- test coverage at the contract level,
- one working demo path,
- one operational owner,
- one measurable product outcome.

## Priority 0: Scope Cuts

These are removals or de-emphasis tasks. They create clarity faster than new features.

### P0.1 Remove Fake Enterprise Surface From The Pitch

- Remove billing from any product narrative until real billing exists.
- Remove support tickets from any product narrative until they integrate with a real workflow.
- Remove policy management from the narrative until rules are enforced at runtime.
- Remove healthcare and compliance-first claims from README, demos, and docs.
- Remove "SOC2" wording from exports unless mapped to actual control evidence.

### P0.2 Fix Repo Credibility

- Restore or intentionally remove the deleted `console/` product from the repo story.
- Replace the one-line root README with a real product and architecture overview.
- Remove broken or stale demos.
- Align all SDK examples with the actual shipped public API.

### Exit Criteria

- A new visitor can tell what the product is in under 60 seconds.
- No demo script calls nonexistent SDK methods.
- No public-facing doc overclaims what the software currently does.

## Priority 1: Core Platform Re-Architecture

This is the make-or-break work.

### P1.1 Define Canonical Data Ownership

Decide and document what is authoritative in:

- Postgres,
- Redis,
- object storage,
- in-memory engine cache.

### P1.2 Replace Embedded Single-Node Ownership

- Move orgs, users, session metadata, invitations, exports, access logs, activity, notifications, and billing metadata into Postgres.
- Move rate limiting and idempotency storage into Redis.
- Move snapshots and export artifacts into object storage.
- Keep the reasoning engine as a computation layer, not the system of record.

### P1.3 Make API Instances Stateless

- Remove process-local ownership assumptions from `api.Server`.
- Make session engine loading explicit and cache-based.
- Add cache invalidation and rebuild semantics for multiple API instances.
- Replace local SSE assumptions with a shared event transport if streaming is retained.

### P1.4 Stop Full-Scan Search

- Replace org-wide session scans and history scans with indexed search.
- Add searchable metadata tables for sessions, facts, and activity.
- Define which fields are searchable and which remain blob payloads only.

### Exit Criteria

- Two API instances can serve the same org safely.
- Node restart does not cause correctness loss.
- Search complexity is not proportional to all sessions in the org.

## Priority 2: Productize One Real Workflow

Do not build more platform tabs. Build one workflow end to end.

### Recommended First Workflow

"Approval integrity"

Example:

- an agent prepares an approval recommendation,
- the recommendation depends on facts and policy state,
- if any upstream fact changes, the recommendation is invalidated before execution,
- the system records provenance and exposes why the prior recommendation is no longer valid.

### P2.1 Define The Domain Model

Add explicit first-class concepts for:

- decision,
- execution target,
- approval state,
- blocking dependency,
- invalidation reason,
- policy version,
- actor type.

### P2.2 Add Runtime Enforcement

- A stale decision must not only be visible as invalid.
- It must be blocked from execution.
- Add a gating API that answers: "Can this decision still be executed?"
- Add machine-readable invalidation reasons.

### P2.3 Add Workflow-Specific Read Models

Build read models for:

- active decisions,
- invalidated decisions,
- pending approvals,
- blocked executions,
- decision lineage,
- exception queue.

### Exit Criteria

- The product can run one approval workflow end to end without hand-waving.
- Invalidation changes operational behavior, not just graph state.

## Priority 3: SDK And Contract Reliability

The SDKs are currently wrapper-shaped. They need to become real product surfaces.

### P3.1 Establish A Public API Contract

- Write versioned OpenAPI spec that matches actual behavior.
- Mark stable versus experimental routes.
- Remove undocumented routes or document them properly.

### P3.2 Add Contract Tests

- Python SDK against live test server
- TypeScript SDK against live test server
- auth flows
- idempotency behavior
- backpressure behavior
- invalidation and explanation flows

### P3.3 Clean SDK Surfaces

- Keep session operations only on session objects.
- Remove or fix examples that imply client-level methods that do not exist.
- Make error types explicit and consistent.
- Ensure async and sync clients have parity.

### P3.4 Demo Discipline

Each demo must:

- have one command path,
- run from a clean checkout,
- use public SDK calls only,
- target a real workflow.

### Exit Criteria

- Demos run cleanly.
- SDKs are consistent with the server.
- Public contract changes are caught by CI.

## Priority 4: Security And Operational Hardening

This backlog assumes the product may later target higher-trust environments. The basics cannot remain casual.

### P4.1 Auth And Secret Flows

- Replace logged password reset tokens with real delivery flow or disable password reset until implemented.
- Treat invitation tokens as secrets and audit their lifecycle.
- Add secret rotation guidance for JWT and service-level keys.

### P4.2 Distributed Rate Limiting

- Replace per-key timestamp arrays with Redis-backed token bucket or sliding window.
- Support tenant-level and key-level rate policies.
- Return predictable retry headers.

### P4.3 Audit And Retention

- Make retention settings enforce deletion or archival, not just filtered reads.
- Ensure audit logs are immutable and exportable.
- Separate application logs from customer audit records.

### P4.4 Backup And Restore

- Redesign backup and restore for shared-store architecture.
- Support scoped restore strategy.
- Test disaster recovery, not just happy-path backup round-trip.

### Exit Criteria

- Security-sensitive flows are not handled through logs or local-only state.
- Retention and audit behavior are operationally real.

## Priority 5: Explainability That Matters To Customers

The explanation engine is one of the strongest parts of the repo. Make it valuable to operators, not just elegant in code.

### P5.1 Turn Explanations Into Actionable Operator Output

- Add "why blocked" summaries for decisions.
- Add "what changed" diffs between valid and invalid states.
- Add "minimum dependency set" for a decision.
- Add policy version and source attribution to explanation payloads.

### P5.2 Add Replay For Incident Review

- Support point-in-time replay for a decision.
- Support reconstructing the exact fact set at execution attempt time.
- Support counterfactual review from the UI or API.

### Exit Criteria

- An operator can understand why a decision was blocked without reading raw graph data.
- Explanations reduce debugging time in a measurable way.

## Priority 6: Data Model And Storage Hygiene

The current code stores too much loosely typed JSON in too many places.

### P6.1 Normalize High-Value Entities

Introduce typed schemas for:

- decisions,
- facts metadata,
- sessions,
- workflow runs,
- audit records,
- policy attachments,
- export jobs.

### P6.2 Leave Payload Blobs Where They Belong

Use blobs for:

- raw fact payloads,
- full explanation snapshots,
- large export payloads.

Do not use blobs for:

- search,
- access control,
- joins,
- workflow status.

### Exit Criteria

- Query paths do not depend on blob scans.
- Core product entities have stable schemas.

## Priority 7: Observability And SLOs

You already have metrics. They need to become product and platform controls.

### P7.1 Define SLOs

- decision write latency,
- invalidation propagation latency,
- explanation generation latency,
- error rate,
- stale-decision block success rate,
- session load latency.

### P7.2 Add Structured Domain Metrics

- decisions_created_total
- decisions_invalidated_total
- stale_execution_blocks_total
- explanation_requests_total
- explanation_replay_latency_ms
- workflow_incidents_prevented_total

### P7.3 Add Load Profiles

Test:

- many tenants, few sessions each
- few tenants, many sessions each
- hot-session burst writes
- invalidation storms
- export concurrency

### Exit Criteria

- Platform health is visible through customer-relevant metrics.
- You can prove the product blocks stale decisions under load.

## Priority 8: Evaluation And Defensibility

This is how the company becomes harder to copy.

### P8.1 Build A Reasoning-Failure Dataset

Capture anonymized classes of:

- stale decisions,
- broken dependency chains,
- false-valid decisions,
- invalidation timing failures,
- explanation failures.

### P8.2 Build A Standard Evaluation Harness

Measure:

- invalidation correctness,
- explanation correctness,
- replay correctness,
- cross-instance consistency,
- false block rate,
- missed block rate.

### P8.3 Publish Internal Benchmarks

Benchmark against:

- prompt-only memory approaches,
- generic vector memory,
- naive database-backed fact stores.

### Exit Criteria

- You have proprietary evaluation assets tied to customer pain.
- You can explain why a competitor cannot trivially replicate the outcome.

## Priority 9: Concrete Backlog By Milestone

### Milestone A: Repo Credibility

- Replace root README
- Fix demos
- Remove fake enterprise pitch language
- Add SDK contract tests
- Add architecture ADR for shared-state migration

### Milestone B: Multi-Instance Core

- Postgres schema for users, orgs, sessions, activity, access logs, exports
- Redis rate limiting and idempotency
- engine cache abstraction
- session loader abstraction
- searchable session and fact indexes

### Milestone C: Approval Integrity V1

- decision entity
- execution gating API
- invalidation reason model
- decision lineage read model
- blocked execution event
- operator explanation endpoint

### Milestone D: Operational Proof

- load tests
- SLO dashboards
- retention jobs
- disaster recovery test
- design partner pilot instrumentation

## What To Build Last

- broad org admin UI
- billing
- support ticket system
- horizontal feature tabs
- domain expansion to healthcare
- fancy export/reporting layers

These only matter after the core workflow proves value.

## Definition Of Done For The Company, Not Just The Code

The product is ready for a serious fundraising story when:

- it supports real multi-tenant deployment,
- one workflow has repeatable customer pull,
- the product blocks stale decisions in production,
- operators use explanations during real incidents,
- and you can show measurable economic value from prevented bad actions.

Until then, every engineering choice should be judged by one question:

"Does this help us own one painful workflow where stale agent reasoning causes real damage?"

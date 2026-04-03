# V1 Real-World Enforcement Addendum

This document is additive to:

- `V1_IMPLEMENTATION_SPEC.md`
- `ENGINEERING_BACKLOG.md`
- `DUE_DILIGENCE_FIX_PLAN.md`

It does not replace those documents.

Its purpose is to close the gap in three areas that were still under-specified:

- ingestion of real-world state changes,
- declarative dependency mapping from business logic into the graph,
- active interruption of AI-assisted execution when facts go stale.

## Why This Addendum Exists

Without this layer, the product is still too close to observability software.

It can explain stale reasoning, but it is not yet strong enough to:

- hear real system changes fast enough,
- translate them into enforceable graph facts reliably,
- and interrupt execution before an AI agent completes a bad approval.

That is the difference between a vitamin and a painkiller.

## Design Principles

These principles are mandatory for this addendum:

- raw external events are preserved immutably before normalization,
- normalized facts are deterministic and idempotent,
- business dependencies are declared, not hardcoded per customer,
- hot-path invalidation must outrun execution,
- terminal approvals must leave the hot graph and move to archival read models,
- execution must require a fresh server-issued check, not trust an earlier local decision.

## 1. Real-World Ingestion Layer

The current system assumes facts are already present.

That is not enough for the wedge.

V1 needs a first-class ingestion layer that accepts external events and turns them into immutable internal facts.

## Ingestion Goals

The ingestion layer must:

- receive webhook events from external systems,
- authenticate and verify the source,
- persist the raw event immediately,
- normalize the source payload into one or more internal facts,
- write facts idempotently into the relevant session or sessions,
- trigger invalidation and downstream interruption if those facts affect active decisions.

## Ingestion Architecture

Use a staged ingestion pipeline:

1. receive the external webhook,
2. verify signature and source identity,
3. persist the raw payload and headers,
4. acknowledge quickly,
5. normalize into canonical internal events,
6. resolve target session or subject mapping,
7. emit immutable fact writes,
8. trigger dependency re-evaluation and interruption.

This must not be implemented as one giant handler with source-specific logic embedded in `api/server.go`.

## New API Surface

Add these endpoints:

- `POST /v1/ingest/webhooks/{source}`
- `POST /v1/ingest/events/{event_id}/replay`
- `GET /v1/ingest/events/{event_id}`
- `GET /v1/ingest/sources`

These routes are for system-to-system ingestion, not operator UI traffic.

## New Data Model

Add these tables:

- `ingest_sources`
- `webhook_endpoints`
- `raw_events`
- `normalized_events`
- `subject_mappings`
- `fact_writes`
- `delivery_failures`

### raw_events

Store:

- source name,
- external event ID,
- signature metadata,
- headers,
- raw body,
- receive timestamp,
- verification status,
- replay count.

### normalized_events

Store:

- normalized event ID,
- source,
- external event ID,
- event type,
- subject key,
- normalized payload,
- canonical event time,
- processing status.

### subject_mappings

This is how external identity becomes internal graph identity.

Store:

- source,
- external subject ref,
- internal subject ref,
- org ID,
- workflow type,
- active flag.

## Canonical Event Envelope

Every external system must normalize to one canonical event envelope before fact emission:

```json
{
  "event_id": "evt_123",
  "source": "okta",
  "event_type": "user_role_changed",
  "org_id": "org_1",
  "subject_ref": "user:12345",
  "occurred_at": "2026-04-03T10:20:30Z",
  "attributes": {
    "old_role": "contractor",
    "new_role": "employee"
  },
  "source_ref": {
    "external_id": "00u123",
    "trace_id": "abc"
  }
}
```

The graph should consume canonical envelopes, not source-native payload chaos.

## Fact Emission Rules

Normalization outputs must create immutable facts with:

- stable fact IDs,
- source metadata,
- event timestamps,
- source payload hashes,
- policy version if relevant,
- explicit causal category.

Examples:

- `subject.user_status.active`
- `subject.user_role.employee`
- `policy.access.max_sensitivity.internal_only`
- `approval_context.budget_policy.current`

Facts should be immutable observations. Corrections create new events or invalidations, not in-place mutation.

## Latency Strategy

The ingestion layer needs two processing modes:

### Standard Mode

- persist raw event,
- acknowledge fast,
- normalize asynchronously,
- update graph within a bounded SLA.

Use this for non-critical state changes.

### Enforcement-Critical Mode

- persist raw event,
- run deterministic normalization inline or with near-immediate worker dispatch,
- emit facts before acknowledgement if the source change can invalidate live approvals.

Use this for:

- user status changes,
- permission changes,
- manager or approver changes,
- policy version changes,
- exception revocations.

## Ingestion Latency Budget

Set explicit targets:

- raw event persisted in under 100 ms p95,
- acknowledgement in under 250 ms p95,
- normalized fact emission in under 1 second p95 for critical events,
- interruption dispatch in under 2 seconds p95 after event receipt.

If the product cannot hit this, it cannot claim real-time stale-decision prevention.

## Ticket Group A: Ingestion Layer

### Ticket V1-801: Add Ingestion Contracts

Scope:

- create webhook ingestion routes,
- verify source signatures,
- persist raw events before further processing.

Files:

- `api/ingest_contracts.go` new
- `api/models/ingest.go` new
- `store/interfaces.go`
- `store/postgres/`

Acceptance criteria:

- external events are durably recorded before normalization,
- duplicate external event IDs are idempotent,
- invalid signatures are rejected and audited.

### Ticket V1-802: Add Source Normalization Pipeline

Scope:

- create source-specific normalizers,
- map raw payloads into canonical event envelopes,
- emit immutable facts.

Files:

- `ingest/normalizers/` new
- `ingest/pipeline.go` new
- `store/interfaces.go`
- `core/engine.go`

Acceptance criteria:

- source payloads never reach the core engine unnormalized,
- canonical envelopes are deterministic,
- normalization failures are retryable and observable.

### Ticket V1-803: Add Subject Resolution

Scope:

- map external user or object references to internal subject references and sessions,
- support multiple active workflows per subject.

Files:

- `ingest/resolution.go` new
- `store/postgres/`
- `api/ingest_contracts.go`

Acceptance criteria:

- one external identity can resolve to the correct internal workflows,
- unmatched events are quarantined rather than silently dropped.

## 2. Declarative Dependency Mapping

The engine currently understands facts and invalidation mechanics.

What it does not yet have is a formal configuration layer that defines:

- which facts matter to which decision types,
- what change should invalidate a decision,
- and how external events translate into those causal dependencies.

That needs to be declared, versioned, and testable.

## Dependency Mapping Goals

The config layer must let the system express rules like:

"Approval X remains executable only if User Status, Approver Chain, Policy Version, and Budget State are unchanged."

This must not require hardcoding every customer workflow in Go.

## Configuration Shape

Add a declarative dependency pack format.

Start with YAML or JSON.

Each pack should define:

- workflow type,
- decision type,
- relevant subject selectors,
- required fact categories,
- invalidation rules,
- terminal conditions,
- archival rules,
- interruption severity.

## Example Dependency Pack

```yaml
workflow_type: access_approval
decision_type: grant_access
required_facts:
  - fact_key: subject.user_status.active
    required_status: valid
    invalidates_on_change: true
  - fact_key: subject.manager.current
    required_status: valid
    invalidates_on_change: true
  - fact_key: policy.access.version.current
    required_status: valid
    invalidates_on_change: true
execution_policy:
  requires_fresh_check: true
  max_check_age_seconds: 30
interruption_policy:
  mode: immediate
  callback_required: true
archive_policy:
  archive_after_terminal_hours: 24
  retain_read_model_days: 365
```

## Compiler Requirement

The dependency pack must be compiled and validated before use.

Compilation should check:

- unknown fact keys,
- impossible dependency combinations,
- missing execution policy,
- invalid archive policy,
- missing interruption config for enforcement-critical workflows.

Do not interpret arbitrary customer config at runtime without validation.

## Graph Lifecycle Strategy

This is the missing answer to the memory-thrashing problem.

The hot graph cannot retain every resolved approval forever.

Use a lifecycle model:

### Hot State

Keep only:

- active sessions,
- recently mutated sessions,
- decisions pending execution,
- decisions with open interruption windows.

### Warm State

Persist:

- decision records,
- dependency edges,
- latest fact metadata,
- explanation summary,
- execution checks,
- audit trail.

This is queryable without loading the full graph.

### Archived State

For terminal decisions:

- store snapshot summary,
- store dependency summary,
- store explanation output,
- store execution result,
- evict the in-memory graph,
- archive cold payloads if needed.

This allows future audit and replay without keeping the graph hot.

## Pruning Rules

A graph becomes eligible for compaction when:

- all decisions in the session are terminal,
- no interruption callbacks are pending,
- no ingestion retries are still open,
- retention rules allow compaction,
- and the session has been inactive longer than the hot TTL.

## Compaction Output

Compaction must produce:

- terminal decision summary,
- dependency summary,
- explanation summary,
- execution audit summary,
- latest snapshot pointer,
- archive timestamp.

After compaction:

- the session engine may be evicted,
- the decision read model remains queryable,
- replay is still possible from stored history and snapshot artifacts.

## Ticket Group B: Dependency Mapping And Pruning

### Ticket V1-811: Add Dependency Pack Schema

Scope:

- define the dependency pack format,
- add parser and validator,
- version the pack schema.

Files:

- `config/dependency_pack.go` new
- `config/dependency_pack_test.go` new
- `docs/dependency-packs.md` new

Acceptance criteria:

- invalid packs fail fast,
- valid packs can be loaded deterministically,
- packs are versioned and testable.

### Ticket V1-812: Bind Decisions To Dependency Packs

Scope:

- require each decision type to reference a dependency pack version,
- persist pack version with the decision.

Files:

- `api/models/decision.go`
- `api/decision_contracts.go`
- `store/interfaces.go`
- `store/postgres/`

Acceptance criteria:

- every created decision records which dependency rules governed it,
- future explanations can cite the exact rule pack version.

### Ticket V1-813: Add Session Compaction And Archival Jobs

Scope:

- compact terminal sessions,
- write archival summaries,
- evict hot graph state safely.

Files:

- `jobs/compaction.go` new
- `store/postgres/`
- `api/server.go`
- `store/interfaces.go`

Acceptance criteria:

- terminal sessions leave the hot cache automatically,
- decision read APIs still work after compaction,
- replay remains possible from stored artifacts.

### Ticket V1-814: Add Hot-State Eligibility Rules

Scope:

- formalize which sessions stay hot,
- stop heap-based eviction from being the main policy.

Files:

- `api/server.go`
- `jobs/compaction.go`
- `store/interfaces.go`

Acceptance criteria:

- eviction is policy-driven, not only memory-pressure driven,
- pending or interruptible sessions are never pruned early.

## 3. Interruption Protocol

This is the enforcement layer.

It is not enough to mark a decision invalid in storage.

The system must be able to halt or reject execution in time.

## Interruption Goals

When an upstream fact changes, the system must:

- invalidate affected decisions,
- revoke any stale execution authorization,
- notify the execution layer immediately,
- and reject any later execution attempt that still slips through.

That requires both push and pull enforcement.

## Required Protocol Design

V1 should use a dual-layer protocol:

### Pull Enforcement

No AI agent may execute a decision without a fresh server-issued execution check.

The check returns:

- `decision_id`
- `decision_version`
- `executable`
- `reason_codes`
- `execution_token`
- `expires_at`

Execution must then call the server again with the `execution_token`.

The server must verify:

- the token is unexpired,
- the decision version has not changed,
- the dependency state is still valid.

### Push Enforcement

On invalidation, the system should emit:

- outbound webhook callback,
- or queue/event notification,
- or both.

The callback payload should include:

- decision ID,
- session ID,
- old decision version,
- interruption reason codes,
- invalidated fact IDs,
- occurred-at timestamp,
- idempotency event ID.

## Race Condition Strategy

This is the critical design rule:

Do not trust a previously generated approval recommendation.

Always require a fresh execution lease.

Use this flow:

1. agent creates or reads a decision,
2. agent requests `execute-check`,
3. server returns short-lived `execution_token` and `decision_version`,
4. if an upstream fact changes, server increments decision version and revokes the token,
5. interruption callback is pushed immediately,
6. any subsequent `execute` call with the old token fails.

This is how the system beats the race even if the LLM has already generated text.

The text can exist. The execution still fails.

## Token Rules

Execution tokens must be:

- short lived,
- single decision scoped,
- single use,
- version bound,
- auditable.

Recommended initial TTL:

- 15 to 30 seconds.

## Outbound Delivery Guarantees

Callbacks must support:

- signed payloads,
- retry with backoff,
- idempotent event IDs,
- dead-letter recording after repeated failure,
- delivery audit logs.

Do not assume the receiver is always online.

## Interruption Severity Levels

The protocol should support:

- `notify_only`
- `block_and_notify`
- `block_notify_and_escalate`

V1 should default enforcement-critical workflows to `block_and_notify`.

## Ticket Group C: Interruption Protocol

### Ticket V1-821: Add Execution Token Model

Scope:

- add execution check tokens,
- bind tokens to decision version and expiry,
- invalidate tokens on upstream fact changes.

Files:

- `api/models/decision.go`
- `api/decision_contracts.go`
- `store/interfaces.go`
- `store/redis/`

Acceptance criteria:

- stale execution tokens fail reliably,
- a decision version change invalidates prior tokens immediately.

### Ticket V1-822: Add Outbound Interruption Delivery

Scope:

- send interruption callbacks on decision invalidation,
- persist delivery attempts and outcomes.

Files:

- `interrupts/delivery.go` new
- `api/decision_contracts.go`
- `store/postgres/`
- `store/interfaces.go`

Acceptance criteria:

- every interruption has a durable delivery record,
- retries are automatic,
- callbacks are signed and idempotent.

### Ticket V1-823: Add Pull And Push Race Tests

Scope:

- simulate an approval recommendation being generated while an upstream fact changes,
- verify stale execute tokens fail,
- verify callbacks are emitted.

Files:

- `tests/e2e/interruption_race_test.go` new
- `tests/contract/`

Acceptance criteria:

- the stale execution path fails consistently,
- interruption callbacks are observable,
- tests prove the system does not rely on timing luck.

## 4. Operational Metrics For This Addendum

If these metrics are not tracked, the team will not know whether the wedge really works.

Track:

- webhook receive latency,
- normalization latency,
- fact emission latency,
- invalidation propagation latency,
- interruption dispatch latency,
- stale execute rejection count,
- callback delivery success rate,
- callback retry count,
- session compaction count,
- hot session count,
- archived session count.

## 5. Definition Of Done For The Addendum

This addendum is complete only when all of the following are true:

- real webhook events can enter the system,
- raw events are preserved before normalization,
- normalized events produce immutable facts,
- dependency rules are declared in versioned packs,
- execution requires a fresh token-bound check,
- invalidation revokes stale execution authorization,
- interruption callbacks are delivered with retries and auditability,
- terminal sessions leave hot memory through compaction,
- archived decisions remain queryable without full graph replay.

## Fresh-Chat Prompt Addendum

If you want a future implementation chat to include this scope, append the following to the implementation prompt:

```text
Also implement the real-world enforcement addendum in /home/fluxx/Workspace/casualdb/V1_REAL_WORLD_ENFORCEMENT_ADDENDUM.md.

Do not replace the existing V1 plan. Extend it.

Additional priorities:
1. Add webhook ingestion contracts, raw event storage, normalization pipeline, and subject resolution.
2. Add dependency-pack configuration and bind decisions to pack versions.
3. Add compaction and archival so terminal sessions leave hot graph memory safely.
4. Add execution tokens plus interruption callbacks so invalidation can beat execution races.
5. Add metrics and tests for ingestion latency, interruption delivery, and stale-token rejection.
```

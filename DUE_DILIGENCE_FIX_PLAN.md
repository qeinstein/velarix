# Due Diligence Fix Plan

This document turns the current "Pass" reasons into a concrete operating plan.

It is not a wishlist. It is a sequence of changes required to make this repo credible as a company.

## Why This Gets A Pass Today

- The product is architected like a single-node embedded database service, not a multi-tenant company-ready platform.
- The codebase exposes a large "enterprise" surface area, but much of it is CRUD-only and not connected to real workflow enforcement.
- The market story says healthcare and compliance, while the implemented product is really an agent-state and reasoning utility for developers.
- The SDKs, demos, and repo state are not internally consistent.
- The moat is thin. The interesting core is the invalidation engine. Most of the rest is adapter and wrapper code.

## Code Evidence To Anchor The Plan

- Single-process authoritative state in `main.go`, `api/server.go`, and `store/badger.go`
- Heap-driven session eviction in `api/server.go`
- Local embedded persistence and per-instance caches in `store/badger.go` and `api/server.go`
- CRUD-only admin surfaces in `api/console_contracts.go`
- Fake compliance language in `api/export_builder.go`
- Undefined legal docs in `api/console_contracts.go`
- Demo and SDK mismatch in `demo/agent_pivot.py` and `sdks/python/velarix/client.py`

## 1. Fix The Technical Death Trap

### Current Failure Mode

The API process holds authoritative live session state in RAM, loads whole sessions into memory, persists to local Badger, and evicts based on heap pressure. That is acceptable for a local sidecar or single-node appliance. It is not acceptable for a cloud product expected to scale, survive node loss, or support horizontal concurrency.

### Required Architectural Change

Move from "single process with embedded storage" to "stateless API layer over shared durable state."

### Non-Negotiable Changes

- Make the API layer stateless.
- Move authoritative org, user, session, history, and export metadata into a shared transactional store.
- Move rate limits, idempotency records, and write concurrency controls out of local process memory and local Badger.
- Keep the core reasoning engine as an execution component, not as the system of record.
- Introduce a shared event or pubsub layer for session events, invalidations, and stream fanout.
- Treat in-memory session engines as caches that can be rebuilt safely, not as the canonical source of truth.

### Practical Target Stack

- Postgres for orgs, users, session metadata, indexes, exports, invitations, billing metadata, and searchable history metadata
- Object storage or large-blob storage for snapshots and large export payloads
- Redis for rate limits, idempotency, distributed locks, and hot session cache coordination
- NATS, Redis Streams, or Kafka for cross-instance event fanout if live streams remain part of the product

### What Must Break Out Of Badger

- User and organization records
- Session catalog and org search
- Access logs and activity feeds
- Invitations, notifications, billing, and support tickets
- Rate-limit windows
- Idempotency replay records
- Export jobs and artifacts

### What Can Stay As Core IP

- The invalidation engine
- The justification graph model
- The explanation and counterfactual APIs
- Snapshot serialization logic, once it is no longer tied to single-node ownership

### Done Means

- Two API instances can serve the same tenant without correctness drift.
- A node restart does not require replaying every hot session from local disk.
- Search does not depend on scanning every session history.
- The product can support far more than 10,000 sessions per org without silent degradation.

## 2. Kill Enterprise Theater

### Current Problem

Large parts of the enterprise-looking surface are present in the API, but not present as a real product capability.

### Examples

- Integrations are stored, not executed.
- Policies are stored, not enforced.
- Billing is editable metadata, not billing.
- Support tickets are local records, not a support workflow.
- Legal endpoints return placeholder strings.
- "SOC2 Compliance Export" is branding text, not evidence of an audited control system.

### Fix

Pick one of these two paths for every such endpoint:

- Implement it as a real workflow with side effects, ownership, and test coverage.
- Remove it from the product surface until it is real.

### Immediate Cleanup

- Remove or hide billing, support, and policy endpoints from any pitch until they do something real.
- Remove healthcare and compliance claims from marketing/demo language unless there is actual deployment evidence.
- Replace placeholder legal endpoints with actual docs or remove those routes.
- Remove "SOC2" terminology from exports unless backed by a formal control framework and evidence trail.

## 3. Make The Repo Internally Consistent

### Current Problem

A pitchable startup repo cannot have its console app deleted in the working tree, demos that call nonexistent SDK methods, and a mismatch between the marketed flow and the shipped API.

### Required Fixes

- Restore the console app or stop pitching a console product.
- Make all demos run against the actual shipped SDK and API.
- Remove dead or stale demo scripts immediately if they are not maintained.
- Add contract tests for Python and TypeScript SDKs against a live test server.
- Version the APIs and SDKs as real public surfaces, not as informal wrappers.
- Ensure the root README explains the actual current product, not a teaser line.

### Demo Standard

Every demo in `demo/` must:

- run from a clean checkout,
- use the public SDK exactly as shipped,
- target real endpoints that exist,
- have one command path documented and tested.

## 4. Implement Real Security And Operations

### Current Problem

Security is treated seriously in some areas and casually in others. That inconsistency is dangerous.

### Fixes

- Password reset must use an out-of-band delivery path. Never log reset tokens.
- Invitation tokens should be handled like secrets, not casual API payloads.
- Rate limiting must not depend on local serialized timestamp arrays in one node.
- Audit records should be immutable and queryable across instances and over retention windows.
- Backup and restore should be tenant-aware, tested for disaster recovery, and not rely on a single embedded local DB path.
- Access logs and retention settings need real deletion or archive jobs, not just filtered reads.

### Compliance Rule

Do not market regulated-vertical readiness until the following are true:

- real retention enforcement exists,
- real access-control boundaries exist,
- legal documents are defined,
- audit exports map to actual controls,
- deployment and incident operations are documented and tested.

## 5. Narrow The Product To A Real User

### What The Code Actually Is

The strongest implemented concept is a reasoning-state layer for long-lived agents:

- facts,
- justification sets,
- invalidation,
- explanation,
- replay,
- provenance capture,
- framework adapters.

That is a developer or platform product, not yet a healthcare workflow product.

### Recommended ICP

Target teams deploying internal AI agents where stale state creates real operational risk:

- financial operations,
- support automation,
- procurement and approvals,
- internal compliance workflows,
- regulated back-office decisioning.

### Avoid This Trap

Do not sell "memory for agents."

Sell:

- stale-decision prevention,
- replayable decision provenance,
- invalidation of downstream actions after upstream facts change,
- auditability for long-lived agent workflows.

## 6. What Would Make This A Strong Company

### A Strong Company Here Is Not

- a generic memory layer,
- a framework plugin,
- a nicer wrapper around OpenAI or LangChain,
- a fake enterprise console with many tabs.

### A Strong Company Here Would Be

- the decision-integrity system of record for long-lived AI workflows,
- sold into teams that lose money or take risk when agent state goes stale,
- measured by prevention of bad actions, not by number of stored facts.

### The Conditions Required

- Own a painful workflow.
- Have a narrow initial wedge.
- Deliver a measurable ROI metric.
- Become embedded in customer operations, not just in developer experimentation.
- Accumulate proprietary data and evaluations from real reasoning traces.
- Build distribution through infrastructure, workflow systems, and compliance owners.

### The Best Company Thesis Available From This Repo

"We prevent stale or invalid agent reasoning from reaching production workflows, and we provide replayable decision provenance when facts change."

That is coherent.

"We are the trust layer for autonomous healthcare" is not yet coherent from this codebase.

### What Creates Real Defensibility

- Proprietary datasets of reasoning failures, invalidation cases, and decision trace outcomes
- Deep workflow integrations into systems of record where decisions actually execute
- Domain-specific policy packs and enforcement logic that are hard to replicate quickly
- Proven production benchmarks showing reduced incident rates, reduced audit time, and faster root-cause analysis
- Distribution inside teams already running high-stakes automation

## 7. Recommended Product Strategy

### Path To Credibility

1. Stop pretending the product is broader than it is.
2. Make the core engine production-grade.
3. Pick one painful workflow where stale reasoning has obvious cost.
4. Prove measurable value with design partners.
5. Build adjacent features only after the core decision-integrity wedge is working.

### Best Near-Term Wedge

The most credible near-term company is:

"decision provenance and invalidation infrastructure for enterprise AI agents in regulated internal workflows."

Not healthcare-first.
Not generic agent memory.
Not a multi-tab enterprise control panel.

## 8. 30 / 90 / 180 Day Plan

### 30 Days

- Remove or hide fake enterprise routes from the pitch.
- Restore repo consistency across README, demos, SDKs, and live API.
- Ship a written architecture migration plan off embedded single-node storage.
- Add contract tests for Python and TypeScript SDKs.
- Make one canonical end-to-end demo run cleanly.

### 90 Days

- Ship shared-store architecture for users, orgs, sessions, history metadata, and rate limits.
- Move search off full scans.
- Support multi-instance API deployment.
- Implement one real policy or governance workflow end to end.
- Land 3 design partners in one narrow workflow category.

### 180 Days

- Prove reliability under production-like load.
- Publish customer metrics on prevented stale decisions or reduced audit time.
- Build one proprietary evaluation suite around reasoning invalidation and replayability.
- Expand only after the first workflow has repeatable pull.

## 9. Things To Stop Doing

- Stop marketing unfinished compliance language.
- Stop shipping placeholder endpoints as product.
- Stop using local embedded persistence as the core cloud architecture.
- Stop pitching a broad enterprise platform before one painful workflow works.
- Stop allowing demos and SDKs to drift from the product.

## 10. The Standard For "Investable"

This becomes interesting when all of the following are true:

- The architecture can support real multi-tenant production use.
- The product surface is smaller and sharper.
- One workflow has clear, quantified value.
- The company can explain why customers cannot simply replace it with prompt logic and a database.
- The team has proof that the product changes outcomes, not just abstractions.

Until then, this is an intriguing engine inside an unconvincing company shell.

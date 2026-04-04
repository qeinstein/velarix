# Architecture

Velarix is built around one core capability:

- prevent stale approval decisions from reaching execution after upstream facts change

## Current Runtime Shape

The codebase currently runs as:

- a Go HTTP API
- per-session in-memory reasoning engines
- local Badger-backed persistence for development and tests
- a shared-store path using Postgres with optional Redis coordination
- SDK clients that talk to the public `/v1` API

## Current Logical Model

- a fact is either asserted directly or derived from other facts
- derived facts use OR-of-AND justification sets
- decisions depend on facts and persist dependency snapshots
- invalidating a root fact collapses downstream reasoning that no longer has valid support
- `execute-check` and `execute` determine whether an action is still safe to run

## Approval Guardrail Flow

1. approval facts are recorded into a session
2. a derived fact represents the recommendation
3. a first-class decision is created from that derived fact
4. dependencies are checked right before execution
5. execution is blocked if any required dependency has gone stale
6. explanation endpoints expose the exact blocking reason

## Current Architectural Risk

The main risk still in the repo is over-reliance on:

- process-local engine ownership
- local Badger assumptions
- replay-heavy rebuild behavior

That is acceptable for local development.

It is not the final production architecture.

## Target Runtime Shape

The target architecture for the product is:

- Postgres as the system of record
- Redis for idempotency, rate limiting, and coordination
- rebuildable in-memory engine caches only
- object storage for large artifacts and snapshots

## Design Rule

Badger is a local adapter.

The product should not be designed around one node remaining warm or authoritative.


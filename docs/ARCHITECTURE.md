# Architecture

Velarix is built around one core capability: invalidate stale reasoning when upstream facts change.

## Current Runtime Shape

The repository currently runs as:
- a Go HTTP API,
- per-session in-memory reasoning engines,
- a local Badger-backed persistence adapter for development and tests,
- SDK clients that use the public `/v1` API.

## Target V1 Shape

The implementation plan for V1 moves toward:
- Postgres for durable system-of-record data,
- Redis for idempotency, rate limiting, and coordination,
- object storage for large artifacts and export outputs,
- rebuildable in-memory engine caches rather than process-local authority.

## Core Reasoning Model

- A fact is either asserted directly or derived from other facts.
- Derived facts use OR-of-AND justification sets.
- Invalidating a root fact removes downstream reasoning that no longer has valid support.
- History and explanation endpoints preserve provenance for debugging and review.

## Storage Reality

Badger is still used in this repo, but it should be treated as a local adapter. It is not the long-term production system of record described in the V1 plan.

## Export Reality

Exports include verification hashes and history snapshots. They are integrity artifacts, not proof of an audited compliance program.

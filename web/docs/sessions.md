---
title: "Sessions"
description: "Learn what a Velarix session is, how state accumulates, what the memory cap means, how replay works, and what happens when a session goes idle."
section: "Core Concepts"
sectionOrder: 2
order: 5
---

# Sessions are the unit of working memory

A session is the namespace that holds:

- facts
- justification sets
- journal history
- session config
- snapshots
- explanations
- decisions

In the API, session-scoped routes live under `/v1/s/{session_id}/...`.

## What accumulates inside a session

As you assert and derive facts, Velarix stores:

- the current in-memory engine state
- journal entries for assertions, invalidations, retractions, reviews, and other side effects
- periodic snapshots
- search documents
- optional semantic embeddings

## The 80,000 fact cap

`core.MaxFactsPerSession` is `80000`.

That is a hard safety cap. The code comment also notes that performance degradation usually starts much earlier, around `50000` facts for complex graphs.

The practical recommendation is to archive long-lived sessions and start new ones before they get that large.

## Replay and snapshots

When a session is loaded:

1. Velarix tries to load the latest snapshot
2. if a snapshot exists, it replays journal entries after the snapshot timestamp
3. otherwise it replays the full session history

This is why a session can be rebuilt even after process restart.

## Idle behavior

The server keeps in-memory engine caches and also tracks `LastAccess`. A background eviction ticker removes idle sessions from memory. The persisted session state remains in the store, so later access rebuilds the engine from snapshot plus journal.

That means "evicted from memory" is not the same thing as "deleted".

## Session config

Each session has a `SessionConfig` with:

- `schema`
- `enforcement_mode`
- `auto_retract_contradictions`

`enforcement_mode` defaults to `strict`.

## Example

Create a session in the full authenticated API:

```bash
curl -sS -X POST http://localhost:8080/v1/org/sessions \
  -H "Authorization: Bearer $VELARIX_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "name":"payment approvals",
    "description":"Production approvals for invoice release"
  }'
```

Inspect its runtime summary:

```bash
curl -sS http://localhost:8080/v1/s/SESSION_ID/summary \
  -H "Authorization: Bearer $VELARIX_API_KEY"
```

That summary reports:

- `id`
- `fact_count`
- `enforcement_mode`
- `schema_set`
- `status`

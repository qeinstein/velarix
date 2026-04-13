---
title: "API: Facts And Sessions"
description: "Reference the session-scoped core API for facts, perceptions, config, slices, history, graph inspection, export jobs, and session lifecycle."
section: "API Reference"
sectionOrder: 3
order: 1
---

# Authentication model

In full mode, these routes require bearer auth. In Lite mode, the router only exposes the session-scoped routes and health/docs/metrics.

## `POST /v1/s/{session_id}/facts`

Assert a root or derived fact.

Request body:

- `id` (`string`, required): fact ID
- `payload` (`object`): application data
- `metadata` (`object`): internal metadata
- `embedding` (`number[]`): optional embedding
- `is_root` (`boolean`, required)
- `manual_status` (`number`): required for roots in practice
- `justification_sets` (`string[][]`): required for derived facts

Response body:

- the stored fact object, including computed fields such as `derived_status`, `resolved_status`, and `valid_justification_count`

Errors:

- `400` invalid JSON, empty justification set, unknown parent, cycle, schema violation
- `403` wrong org for session

Example:

```bash
curl -sS -X POST http://localhost:8080/v1/s/demo/facts \
  -H "Authorization: Bearer $VELARIX_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "id":"vendor_verified",
    "is_root":true,
    "manual_status":1.0,
    "payload":{"summary":"Vendor 17 passed KYB"}
  }'
```

## `POST /v1/s/{session_id}/percepts`

Persist a perception or model-derived root fact.

Request body:

- `id` (`string`, required)
- `payload` (`object`, required)
- `confidence` (`number`): defaults to `0.75`
- `modality` (`string`)
- `provider` (`string`)
- `model` (`string`)
- `embedding` (`number[]`)
- `metadata` (`object`)

Response body:

- the stored fact

Example:

```bash
curl -sS -X POST http://localhost:8080/v1/s/demo/percepts \
  -H "Authorization: Bearer $VELARIX_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "id":"ocr_invoice_total",
    "payload":{"claim":"invoice total is 1250 USD"},
    "confidence":0.82,
    "modality":"vision",
    "provider":"openai",
    "model":"gpt-4.1-mini"
  }'
```

## `POST /v1/s/{session_id}/facts/{id}/invalidate`

Invalidate a root fact.

Request body:

- `reason` (`string`)
- `force` (`boolean`): governance override for admin callers

Response body:

- a JSON status envelope

## `POST /v1/s/{session_id}/facts/{id}/retract`

Retract a fact explicitly.

Request body:

- `reason` (`string`)
- `force` (`boolean`)

Response body:

- `status`
- `fact_id`
- `reason`

## `POST /v1/s/{session_id}/facts/{id}/review`

Set review state on a fact.

Request body:

- `status` (`string`, required): `pending`, `approved`, `waived`, or `rejected`
- `reason` (`string`)

Response body:

- the updated fact

## `GET /v1/s/{session_id}/facts/{id}`

Fetch one fact by ID.

## `GET /v1/s/{session_id}/facts/{id}/impact`

Simulate the downstream impact of invalidating a fact.

Response body:

- `impacted_ids`
- `direct_count`
- `total_count`
- `action_count`
- `epistemic_loss`

## `GET /v1/s/{session_id}/facts`

List facts in a session.

Common query params:

- `cursor`
- `limit`
- `valid_only`

## `GET /v1/s/{session_id}/semantic-search`

Query params:

- `q` (`string`, required)
- `limit` (`integer`, default `10`)
- `valid_only` (`boolean`, default `true`)

Response body:

- array of `SemanticMatch`

## `POST /v1/s/{session_id}/config`

Update session config.

Request body:

- `schema` (`string`): JSON Schema
- `enforcement_mode` (`string`): `strict` or `warn`
- `auto_retract_contradictions` (`boolean`)

Response body:

- the current `SessionConfig`

## `GET /v1/s/{session_id}/config`

Fetch the current `SessionConfig`.

## `POST /v1/s/{session_id}/revalidate`

Replay session history into a fresh engine state.

Response body:

- status information about the rebuilt session

## `GET /v1/s/{session_id}/summary`

Response body:

- `id`
- `fact_count`
- `enforcement_mode`
- `schema_set`
- `status`

## `GET /v1/s/{session_id}/slice`

Query params:

- `format`: `json` or `markdown`
- `query`
- `strategy`
- `max_facts`
- `include_dependencies`
- `include_invalid`
- `max_chars`

Response body:

- JSON array of facts for `format=json`
- markdown text for `format=markdown`

Example:

```bash
curl -sS "http://localhost:8080/v1/s/demo/slice?format=markdown&query=payment%20approval&strategy=hybrid&max_facts=20&include_dependencies=true" \
  -H "Authorization: Bearer $VELARIX_API_KEY"
```

## `GET /v1/s/{session_id}/history`

Return the full session journal.

## `GET /v1/s/{session_id}/history/page`

Query params:

- `cursor`
- `limit`
- `from`
- `to`
- `type`
- `q`

Response body:

- `items`
- `next_cursor`

## `GET /v1/s/{session_id}/events`

Server-sent events stream of change notifications.

## `GET /v1/s/{session_id}/graph`

Return graph nodes and edges for visualization.

## `GET /v1/s/{session_id}/export`

Export the current session state.

## Export job routes

### `POST /v1/s/{session_id}/export-jobs`

Create an export job.

Request body:

- `format` (`string`): `csv` or `pdf`

### `GET /v1/s/{session_id}/export-jobs`

List recent export jobs.

### `GET /v1/s/{session_id}/export-jobs/{id}`

Fetch one job.

### `GET /v1/s/{session_id}/export-jobs/{id}/download`

Download the export artifact after completion.

## Session management routes

### `POST /v1/org/sessions`

Create an empty session record.

Request body:

- `name` (`string`)
- `description` (`string`)

### `GET /v1/org/sessions`

List visible sessions for the org.

### `PATCH /v1/org/sessions/{id}`

Update `name` and `description`.

### `DELETE /v1/org/sessions/{id}`

Archive the session and evict its in-memory cache.

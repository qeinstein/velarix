---
title: "API: Decisions"
description: "Reference the decision lifecycle endpoints for creation, listing, recomputation, execute-check, execution, lineage, and block explanations."
section: "API Reference"
sectionOrder: 3
order: 2
---

# Decision endpoints

These routes live under `/v1/s/{session_id}/decisions`.

## `POST /v1/s/{session_id}/decisions`

Create a decision.

Request body:

- `decision_id` (`string`)
- `fact_id` (`string`)
- `decision_type` (`string`, required)
- `subject_ref` (`string`)
- `target_ref` (`string`)
- `recommended_action` (`string`)
- `policy_version` (`string`)
- `explanation_summary` (`string`)
- `dependency_fact_ids` (`string[]`)
- `metadata` (`object`)

Response body:

- the stored `Decision`

Example:

```bash
curl -sS -X POST http://localhost:8080/v1/s/demo/decisions \
  -H "Authorization: Bearer $VELARIX_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "decision_type":"payment_release",
    "fact_id":"decision.release_payment",
    "subject_ref":"inv-1042",
    "target_ref":"vendor-17",
    "recommended_action":"release payment",
    "dependency_fact_ids":[
      "vendor_verified",
      "invoice_approved",
      "decision.release_payment"
    ]
  }'
```

## `GET /v1/s/{session_id}/decisions`

List session decisions.

Query params:

- `status`
- `subject`
- `from`
- `to`
- `limit`

Response body:

- `{ "items": [...] }`

## `GET /v1/s/{session_id}/decisions/{decision_id}`

Fetch one decision.

## `POST /v1/s/{session_id}/decisions/{decision_id}/recompute`

Rebuild dependency snapshots and refresh status.

Request body:

- `fact_id` (`string`)
- `dependency_fact_ids` (`string[]`)

Response body:

- `decision`
- `check`

## `POST /v1/s/{session_id}/decisions/{decision_id}/execute-check`

Run a fresh execution eligibility check.

Response body:

- `decision_id`
- `session_id`
- `executable`
- `blocked_by`
- `reason_codes`
- `checked_at`
- `decision_version`
- `session_version`
- `expires_at`
- `execution_token`
- `explanation_summary`
- `dependency_snapshots`

Errors:

- `404` decision not found
- `403` wrong org or unauthorized session

## `POST /v1/s/{session_id}/decisions/{decision_id}/execute`

Attempt execution using a current token.

Request body:

- `execution_ref` (`string`)
- `execution_token` (`string`, required in practice)

Success response:

- `decision`
- `check`

Conflict cases:

- token missing or invalid
- token expired
- token session version mismatch
- token decision version mismatch
- dependency changed since check
- decision already executed

Those return `409 Conflict`.

## `GET /v1/s/{session_id}/decisions/{decision_id}/lineage`

Return:

- `decision`
- `dependencies`
- `latest_check`
- `decision_fact`

## `GET /v1/s/{session_id}/decisions/{decision_id}/why-blocked`

Return:

- `decision`
- `check`
- `blocked_by`
- `reason_codes`
- optional `explanation`

## Org-wide decision listing

### `GET /v1/org/decisions`

List decisions across the current org.

### `GET /v1/org/decisions/blocked`

List only blocked org decisions.

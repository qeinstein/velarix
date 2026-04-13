---
title: "Decisions And Execution"
description: "Understand what a decision record is, how dependency snapshots are built, what `execute-check` returns, and how execution tokens are validated."
section: "Core Concepts"
sectionOrder: 2
order: 6
---

# Decisions are first-class records

Velarix does not treat execution as an informal side effect. It stores a `Decision` record plus dependency snapshots and execute-check results.

The key structs are in `store/models_decision.go`.

## What a decision contains

A `Decision` stores:

- `decision_id`
- `session_id`
- `org_id`
- optional `fact_id`
- `decision_type`
- `subject_ref`
- `target_ref`
- `status`
- `execution_status`
- `recommended_action`
- `policy_version`
- `explanation_summary`
- `created_by`
- timestamps
- optional `metadata`

## Dependency snapshots

When a decision is created or recomputed, Velarix builds `DecisionDependency` entries from:

- the explicit `dependency_fact_ids` request field, or
- the transitive dependency set of `fact_id`

Each dependency snapshot stores:

- `fact_id`
- `dependency_type`
- `required_status`
- `current_status`
- `source_ref`
- `policy_version`
- `explanation_hint`
- `entrenchment`
- `review_status`
- `review_required`

## `execute-check`

`POST /v1/s/{session_id}/decisions/{decision_id}/execute-check` computes a fresh `DecisionCheck`.

Important fields:

- `executable`
- `blocked_by`
- `reason_codes`
- `checked_at`
- `decision_version`
- `session_version`
- `expires_at`
- `execution_token`
- `dependency_snapshots`

`blocked_by` is a list of `DecisionBlocker` records that explain exactly which dependency prevented execution.

## Execution tokens

If a check is executable, Velarix issues a signed execution token with a 30-second TTL.

The token binds:

- session ID
- decision ID
- org ID
- decision version
- session version
- check timestamp

Execution fails if any of those no longer match the current state.

## Example

Create:

```bash
curl -sS -X POST http://localhost:8080/v1/s/demo/decisions \
  -H 'Content-Type: application/json' \
  -d '{
    "decision_type":"payment_release",
    "fact_id":"decision.release_payment",
    "subject_ref":"inv-1042",
    "target_ref":"vendor-17",
    "dependency_fact_ids":[
      "vendor_verified",
      "invoice_approved",
      "decision.release_payment"
    ]
  }'
```

Check:

```bash
curl -sS -X POST http://localhost:8080/v1/s/demo/decisions/DECISION_ID/execute-check
```

Execute:

```bash
curl -sS -X POST http://localhost:8080/v1/s/demo/decisions/DECISION_ID/execute \
  -H 'Content-Type: application/json' \
  -d '{
    "execution_ref":"payment-job-782",
    "execution_token":"TOKEN_FROM_EXECUTE_CHECK"
  }'
```

If a dependency changed after the check, the execute call returns `409 Conflict` and the body is the blocking `DecisionCheck`.

---
title: "Quickstart"
description: "Go from an empty machine to a running Velarix session with facts, a decision, an execute-check, and a blocked stale execution."
section: "Getting Started"
sectionOrder: 1
order: 3
---

# Ten-minute quickstart

This page uses Lite mode because it removes auth and shared infrastructure from the loop. In Lite mode the router only exposes the session-scoped reasoning endpoints plus health and metrics.

## 1. Start the server

```bash
export VELARIX_ENV=dev
export VELARIX_BADGER_PATH="$(mktemp -d)"
go run main.go --lite
```

The server listens on `http://localhost:8080` unless `PORT` is set.

## 2. Assert the supporting facts

```bash
curl -sS -X POST http://localhost:8080/v1/s/payment-demo/facts \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "vendor_verified",
    "is_root": true,
    "manual_status": 1.0,
    "payload": {
      "summary": "Vendor 17 passed KYB"
    }
  }'
```

```bash
curl -sS -X POST http://localhost:8080/v1/s/payment-demo/facts \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "invoice_approved",
    "is_root": true,
    "manual_status": 1.0,
    "payload": {
      "summary": "Invoice inv-1042 was approved"
    }
  }'
```

## 3. Derive the recommendation

```bash
curl -sS -X POST http://localhost:8080/v1/s/payment-demo/facts \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "decision.release_payment",
    "is_root": false,
    "justification_sets": [["vendor_verified", "invoice_approved"]],
    "payload": {
      "summary": "Release payment for inv-1042"
    }
  }'
```

## 4. Create a decision

```bash
curl -sS -X POST http://localhost:8080/v1/s/payment-demo/decisions \
  -H 'Content-Type: application/json' \
  -d '{
    "decision_type": "payment_release",
    "fact_id": "decision.release_payment",
    "subject_ref": "inv-1042",
    "target_ref": "vendor-17",
    "recommended_action": "release payment",
    "dependency_fact_ids": [
      "vendor_verified",
      "invoice_approved",
      "decision.release_payment"
    ]
  }'
```

The response contains a `decision_id`. Save it for the next step.

## 5. Run an execute-check

```bash
curl -sS -X POST http://localhost:8080/v1/s/payment-demo/decisions/DECISION_ID/execute-check
```

When the decision is still supported, the response includes:

- `executable: true`
- `blocked_by: []`
- `reason_codes: []`
- `execution_token`
- `expires_at`

## 6. Make the decision stale

Invalidate one of the premises:

```bash
curl -sS -X POST http://localhost:8080/v1/s/payment-demo/facts/vendor_verified/invalidate
```

## 7. Check again

```bash
curl -sS -X POST http://localhost:8080/v1/s/payment-demo/decisions/DECISION_ID/execute-check
```

Now the response should show:

- `executable: false`
- at least one item in `blocked_by`
- a `reason_code` such as `dependency_missing_or_invalid` or `dependency_invalid`

## 8. Ask why

```bash
curl -sS "http://localhost:8080/v1/s/payment-demo/decisions/DECISION_ID/why-blocked"
```

Or explain the underlying decision fact directly:

```bash
curl -sS "http://localhost:8080/v1/s/payment-demo/explain?fact_id=decision.release_payment"
```

## The same flow in Python

```python
from velarix import VelarixClient

client = VelarixClient(base_url="http://localhost:8080")
session = client.session("payment-demo-sdk")

session.observe("vendor_verified", {"summary": "Vendor 17 passed KYB"})
session.observe("invoice_approved", {"summary": "Invoice inv-1042 was approved"})
session.derive(
    "decision.release_payment",
    [["vendor_verified", "invoice_approved"]],
    {"summary": "Release payment for inv-1042"},
)

decision = session.create_decision(
    "payment_release",
    fact_id="decision.release_payment",
    subject_ref="inv-1042",
    target_ref="vendor-17",
    dependency_fact_ids=[
        "vendor_verified",
        "invoice_approved",
        "decision.release_payment",
    ],
)

first_check = session.execute_check(decision["decision_id"])
session.invalidate("vendor_verified")
second_check = session.execute_check(decision["decision_id"])
why_blocked = session.get_decision_why_blocked(decision["decision_id"])
```

## What to read next

- [How Velarix Works](/docs/how-velarix-works)
- [Facts And Belief Graph](/docs/facts-and-belief-graph)
- [Decisions And Execution](/docs/decisions-and-execution)
- [API Facts And Sessions](/docs/api-facts-and-sessions)

# Error Codes

Velarix uses standard HTTP status codes with JSON payloads.

The approval-guardrail workflow depends most on the following responses.

## 400 Bad Request

Common causes:

- invalid JSON
- schema validation failure
- unknown parent fact
- cycle or logical violation
- malformed decision payload

## 401 Unauthorized

Common causes:

- missing API key
- invalid API key
- expired or invalid JWT

## 403 Forbidden

Common causes:

- cross-org access attempt
- missing scope for the route
- admin-only route accessed by non-admin user

## 404 Not Found

Common causes:

- session, fact, or decision does not exist
- dependency read requested for a missing record

## 409 Conflict

This is the most important approval-guardrail failure mode.

Common causes:

- `execute` was called on a stale decision
- one or more required facts are missing or invalid

Expected operator response:

- inspect `reason_codes`
- inspect `blocked_by`
- call `why-blocked` if more detail is needed

## 429 Too Many Requests

Common causes:

- rate limit exceeded for the current key

Expected response behavior:

- honor `Retry-After`
- retry safely with idempotency keys

## 503 Service Unavailable

Common causes:

- per-org write backpressure limit reached

Expected response behavior:

- honor `Retry-After`
- retry safely with the same idempotency key

## Common Troubleshooting Cases

### Decision Is No Longer Executable

Likely cause:

- an upstream fact changed after the decision was created

What to do:

- run `execute-check`
- review `blocked_by`
- call `GET /v1/s/{session_id}/decisions/{decision_id}/why-blocked`

### Fact Does Not Appear In The Slice

Likely cause:

- the fact resolved below the confidence threshold
- one of its required parents was invalidated

What to do:

- fetch the fact directly
- inspect explanation endpoints

### Session Access Feels Slow

Likely cause:

- engine rebuild from persisted state
- large history or snapshot load

What to do:

- verify storage path health
- use the shared-store path for production validation


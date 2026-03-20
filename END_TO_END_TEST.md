# Velarix: End-to-End Production Checklist

This document serves as the final verification suite for Velarix deployments. All items must be validated to ensure reasoning integrity and multi-tenant security.

## 1. Security & Isolation
- [ ] **Strict Tenant Separation**
    - **Method**: Create a session using Org A's key. Attempt to append history or assert a fact into that same session ID using Org B's key.
    - **Pass Criteria**: Org B receives `403 Forbidden` or `404 Not Found`. `OrgID` must be checked on every state-modifying handler.
- [ ] **Mandatory Encryption Enforcement**
    - **Method**: Unset `VELARIX_ENCRYPTION_KEY` and start the server with `VELARIX_ENV=prod`.
    - **Pass Criteria**: Server must `log.Fatal` and refuse to boot without a valid 32-byte key.
- [ ] **Persistent Rate Limiting**
    - **Method**: Trigger 61 requests in 60 seconds. Restart the Go server. Attempt the 62nd request.
    - **Pass Criteria**: 62nd request is still rejected with `429 Too Many Requests` (Quota must persist in Badger).

## 2. Reliability & Recovery
- [ ] **Hybrid Boot Performance**
    - **Method**: Assert 1,000 facts. Trigger a snapshot via internal ticker. Assert 10 more facts. Kill the process and restart.
    - **Pass Criteria**: Engine restores from the snapshot first and then replays only the last 10 journal entries.
- [ ] **Durable Write Sync**
    - **Method**: Assert a critical fact. Force-kill the server process immediately (`kill -9`). Restart.
    - **Pass Criteria**: The fact exists in history. `Sync()` on the journal must guarantee durability.
- [ ] **Idempotent Fact Assertion**
    - **Method**: Send the exact same `AssertFact` JSON payload twice.
    - **Pass Criteria**: Both requests return `201 Created` or `200 OK`. The second request must not create a duplicate or return an error.

## 3. Epistemic Integrity
- [ ] **Causal Collapse Cascade**
    - **Method**: Assert a chain: A -> B -> C. Invalidate Root A.
    - **Pass Criteria**: `GET /v1/s/{id}/facts/C` returns `resolved_status: 0.0`.
- [ ] **Actor Tracking**
    - **Method**: Assert a fact. Fetch the SOC2 export.
    - **Pass Criteria**: The entry must contain the `actor_id` (the Label of the API key used).

## 4. Contract & SDK Compatibility
- [ ] **API Versioning (v1)**
    - **Method**: Access `/v1/sessions`. Access `/sessions` (unversioned).
    - **Pass Criteria**: `/v1/` works. `/` should return `404` for versioned resources to ensure contract discipline.
- [ ] **Python Async Resource Hygiene**
    - **Method**: Initialize `AsyncVelarixClient` and perform 100 concurrent `observe()` calls.
    - **Pass Criteria**: No "Too many open files" errors. Client must reuse a single `httpx.AsyncClient`.

## 5. Observability & Monitoring
- [ ] **Structured Log Parsing**
    - **Method**: Tail server output during an assertion.
    - **Pass Criteria**: Output is valid JSON containing `level`, `msg`, `session_id`, and `actor_id`.
- [ ] **Full Health Check**
    - **Method**: Fetch `GET /health/full`.
    - **Pass Criteria**: Payload includes `badger_connected: true` and real disk usage statistics.

---
*Last Updated: March 20, 2026*

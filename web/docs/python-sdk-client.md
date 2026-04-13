---
title: "Python SDK: Client And Sessions"
description: "Use the synchronous and asynchronous Python clients to create sessions, assert facts, inspect history, run decisions, and work with the Velarix HTTP API."
section: "Python SDK"
sectionOrder: 4
order: 1
---

# Python client

This page covers the core SDK in `sdks/python/velarix/client.py`: `VelarixClient`, `VelarixSession`, `AsyncVelarixClient`, and `AsyncVelarixSession`. These classes are thin HTTP clients with retries, optional local sidecar mode, and a small slice cache.

## Installation

```bash
pip install velarix
```

If you want the client to spawn a local binary in `embed_mode`, the Velarix executable must also be on `PATH` or supplied explicitly with `binary_path`.

## `VelarixClient`

Constructor parameters:

- `base_url` (`str | None`): defaults to `http://localhost:8080` when `embed_mode` is off
- `api_key` (`str | None`)
- `embed_mode` (`bool`): spawn a local sidecar process instead of talking to a remote server
- `binary_path` (`str | None`): sidecar executable path
- `cache_ttl` (`int`): seconds to cache identical slice reads, default `30`
- `max_retries` (`int`): default `5`
- `retry_backoff_base` (`float`): default `0.25`
- `retry_backoff_max` (`float`): default `5.0`
- `timeout_s` (`float`): per-request timeout, default `10.0`

Example:

```python
from velarix.client import VelarixClient

client = VelarixClient(
    base_url="http://localhost:8080",
    api_key="vxk_...",
    cache_ttl=15,
)
session = client.session("demo")
```

### `session(session_id)`

Return a `VelarixSession` bound to an existing or future session ID.

### `create_session(session_id=None)`

Create a session handle locally and initialize it by calling `set_config()`. This does not use the org-scoped `POST /v1/org/sessions` route. It assumes the deployment accepts on-demand session creation when the first session-scoped write arrives.

### `get_sessions()`

Call `GET /v1/sessions` and return visible org sessions.

### `get_usage()`

Call `GET /v1/org/usage` and return aggregate org metrics.

### `list_org_decisions(...)`

Call `GET /v1/org/decisions` with optional filters:

- `status`
- `subject_ref`
- `from_ms`
- `to_ms`
- `limit`

## `VelarixSession`

A session object scopes operations to `/v1/s/{session_id}` and clears its slice cache on writes.

### Fact assertion methods

#### `observe(fact_id, payload=None, idempotency_key=None, confidence=1.0)`

Assert a root fact.

```python
session.observe(
    "vendor_verified",
    {"summary": "Vendor 17 passed KYB"},
    confidence=1.0,
)
```

#### `derive(fact_id, justifications, payload=None, idempotency_key=None)`

Assert a derived fact with OR-of-AND justifications.

```python
session.derive(
    "payment_ready",
    [["vendor_verified", "invoice_approved"]],
    {"summary": "Vendor and invoice checks both passed"},
)
```

#### `record_perception(...)`

Persist a perception-style root fact using the `/percepts` route.

Parameters:

- `fact_id`
- `payload`
- `confidence` default `0.75`
- `modality`
- `provider`
- `model`
- `embedding`
- `metadata`
- `idempotency_key`

### Inspection methods

#### `get_fact(fact_id)`

Fetch one fact.

#### `get_slice(...)`

Fetch a ranked slice in `json` or `markdown` format.

Parameters:

- `format` default `"json"`
- `max_facts` default `50`
- `query`
- `strategy`
- `include_dependencies`
- `include_invalid`
- `max_chars`

The client caches identical slice reads for `cache_ttl` seconds.

#### `get_history()`

Return the full session journal from `GET /history`.

#### `explain(fact_id=None, timestamp=None, counterfactual_fact_id=None)`

Fetch a structured explanation from `/explain`.

#### `semantic_search(query, limit=10, valid_only=True)`

Run semantic or lexical similarity search over session facts.

### Session mutation methods

#### `set_config(schema=None, mode=None, idempotency_key=None)`

Update the session schema and enforcement mode.

The method exposes `schema` and `mode`. The underlying HTTP API also supports `auto_retract_contradictions`, but this client helper does not currently surface that field directly.

#### `revalidate(idempotency_key=None)`

Replay history and rebuild the in-memory engine state.

#### `invalidate(fact_id, idempotency_key=None, reason="", force=False)`

Invalidate a root fact.

#### `retract(fact_id, reason="", idempotency_key=None, force=False)`

Retract a fact.

#### `review_fact(fact_id, status, reason="", idempotency_key=None)`

Set review state to `pending`, `approved`, `waived`, or `rejected`.

#### `extract_and_assert(llm_output, session_context="", auto_retract_contradictions=False)`

Run the extract-and-assert endpoint and return the extraction summary.

### Consistency and reasoning methods

#### `consistency_check(fact_ids=None, max_facts=None, include_invalid=False)`

Run the contradiction detector.

#### `record_reasoning_chain(chain)`

Persist a reasoning chain.

#### `list_reasoning_chains()`

Return stored reasoning chains.

#### `verify_reasoning_chain(chain_id, auto_retract=False)`

Audit a stored reasoning chain.

### Decision methods

#### `create_decision(...)`

Parameters:

- `decision_type` required
- `subject_ref`
- `target_ref`
- `fact_id`
- `decision_id`
- `recommended_action`
- `policy_version`
- `explanation_summary`
- `dependency_fact_ids`
- `metadata`
- `idempotency_key`

#### `list_decisions(status=None, subject_ref=None, from_ms=None, to_ms=None, limit=50)`

Return session decisions.

#### `get_decision(decision_id)`

Fetch one decision.

#### `recompute_decision(decision_id, fact_id=None, dependency_fact_ids=None, idempotency_key=None)`

Refresh dependency snapshots.

#### `execute_check(decision_id, idempotency_key=None)`

Return an execution check, including a short-lived execution token when the decision is executable.

#### `execute_decision(decision_id, execution_ref=None, execution_token=None, idempotency_key=None)`

Execute a decision. If `execution_token` is omitted, the method automatically calls `execute_check()` first. If that check does not return a token, the method raises `ValueError`.

#### `get_decision_lineage(decision_id)`

Fetch decision lineage and dependency snapshots.

#### `get_decision_why_blocked(decision_id)`

Fetch the blocked-by report.

### Compatibility helpers

#### `append_history(event_type, payload=None, fact_id=None, idempotency_key=None)`

This method posts to `/v1/s/{session_id}/history`, but the current Go router intentionally does not expose that write route. Use it only against deployments that add a custom history-write endpoint.

#### `record_decision(kind, payload=None, idempotency_key=None)`

A wrapper around `append_history(...)` that has the same deployment caveat.

#### `delete()`

Archive the session through `DELETE /v1/org/sessions/{session_id}`.

## End-to-end example

```python
from velarix.client import VelarixClient

client = VelarixClient(base_url="http://localhost:8080", api_key="vxk_test")
session = client.session("invoice-demo")

session.observe("vendor_verified", {"vendor_id": "vendor-17"})
session.observe("invoice_approved", {"invoice_id": "inv-1042"})
session.derive(
    "decision.release_payment",
    [["vendor_verified", "invoice_approved"]],
    {"summary": "Release payment for inv-1042"},
)

check = session.create_decision(
    "payment_release",
    fact_id="decision.release_payment",
    subject_ref="inv-1042",
    target_ref="vendor-17",
    dependency_fact_ids=["vendor_verified", "invoice_approved", "decision.release_payment"],
)

print(check["decision_id"])
print(session.execute_check(check["decision_id"]))
```

## Async variants

`AsyncVelarixClient` and `AsyncVelarixSession` expose the same conceptual surface with `async` methods and `httpx.AsyncClient` under the hood.

Use them the same way:

```python
import asyncio
from velarix.client import AsyncVelarixClient

async def main() -> None:
    client = AsyncVelarixClient(base_url="http://localhost:8080", api_key="vxk_test")
    session = client.session("async-demo")
    await session.observe("vendor_verified", {"vendor_id": "vendor-17"})
    fact = await session.get_fact("vendor_verified")
    print(fact["resolved_status"])
    await client.aclose()

asyncio.run(main())
```

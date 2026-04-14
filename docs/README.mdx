# Documentation

Velarix is the decision-integrity service for AI-assisted approvals and operational actions.

The product is built around four guarantees:

- facts are explicit
- support is traceable
- stale conclusions are retractable
- unsafe decisions are blocked before execution

## What Ships In This Repository

- Go API for facts, explanations, slices, governance, and first-class decisions
- symbolic reasoning engine with dependency tracking and negated support
- query-aware belief retrieval for agent runtimes
- Python SDK and runtime helpers
- LangGraph, CrewAI, LlamaIndex, and OpenAI integration surfaces
- `vlx` CLI for core operational workflows
- reproducible contradiction benchmark harness
- maintained demos for approval integrity and framework integrations

## Product Boundary

Velarix is positioned as:

- a decision-integrity service
- an approval-guardrail layer
- a truth-maintenance system for operational AI

Velarix is not positioned as:

- a generic agent-memory platform
- a finished compliance product
- a complete enterprise control plane
- a multi-region hosted service

## Reference Map

- [Architecture](ARCHITECTURE.md)
- [Integration Guide](INTEGRATION_GUIDE.md)
- [Operations](OPERATIONS.md)
- [Security Notes](SECURITY.md)
- [Threat Model](THREAT_MODEL.md)
- [Errors](ERRORS.md)
- [Python SDK](../sdks/python/README.md)
- Workflow-focused OpenAPI document: [`openapi.yaml`](openapi.yaml)
- Broader generated Swagger surface: [`swagger.yaml`](swagger.yaml)

## API Notes

### Fact Fields: `assertion_kind` and `valid_until`

Facts may include `assertion_kind` to scope epistemic contradictions and grounding:
`empirical` (default real-world claim), `uncertain` (hedged real-world claim),
`hypothetical` (conditional/assumed), and `fictional` (story/narrative). Hypothetical
and fictional facts do not contradict empirical facts and cannot ground empirical
derived facts.

Facts may include `valid_until` (unix milliseconds). After expiry, the engine treats
the fact as invalid; an expiry sweep writes `fact_expired` events so dependents are
repropagated immediately and reloads reconstruct the same state.

### Verification And Grounding Controls

Velarix can store verification metadata for facts to reduce the operational impact of
pure model fabrication. Facts may carry:

- `metadata.requires_verification` (bool)
- `metadata.verification_status` (`unverified|verified|rejected`)
- `metadata.verified_at` (unix ms)

When `requires_verification=true`, decisions and execution-critical derived facts can
be configured (via org policies) to require `verification_status=verified` and trusted
`source_type` before they are eligible to ground execution. Admins can update a fact's
verification state via `POST /v1/s/{session_id}/facts/{fact_id}/verify`.

Optional webhook automation: if `VELARIX_VERIFICATION_WEBHOOK_URL` is set, Velarix will
POST verification requests for facts requiring verification and apply the webhook's
response as a `fact_verification` journal event.

### Global Facts (Org-Wide)

Global facts are org-wide ground truths shared across all sessions. Use them for
verified entities or org-level truths that multiple agents/sessions should reference.

Endpoints (org admin):

```bash
# Assert a global fact
curl -sS -X POST "$VELARIX_URL/v1/global/facts" \
  -H "Authorization: Bearer $VELARIX_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"id":"today","is_root":true,"manual_status":1.0,"payload":{"date":"2026-04-13"},"assertion_kind":"empirical"}'

# List global facts
curl -sS -X GET "$VELARIX_URL/v1/global/facts" \
  -H "Authorization: Bearer $VELARIX_API_KEY"

# Get one global fact
curl -sS -X GET "$VELARIX_URL/v1/global/facts/today" \
  -H "Authorization: Bearer $VELARIX_API_KEY"

# Retract a global fact
curl -sS -X DELETE "$VELARIX_URL/v1/global/facts/today" \
  -H "Authorization: Bearer $VELARIX_API_KEY"
```

## Read In Order

1. `README.md`
2. `docs/ARCHITECTURE.md`
3. `api/server.go`
4. `api/decision_contracts.go`
5. `core/engine.go`
6. `store/interfaces.go`
7. `store/badger.go`
8. `store/postgres/`
9. `demo/approval_integrity.py`
10. `tests/e2e_test.go`
11. `tests/reproducibility/hallucination_benchmark.py`

## Canonical References

- The canonical product flow is decision creation plus `execute-check`.
- The canonical demo is `demo/approval_integrity.py`.
- LangGraph and CrewAI are supported surfaces, not the primary product narrative.
- Postgres plus Redis is the production operating path.

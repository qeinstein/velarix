# Velarix: Decision Integrity for AI Agents

![Velarix](https://img.shields.io/badge/Status-Alpha-orange) ![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)

Velarix is the decision-integrity layer for AI-assisted operations.

It keeps recommendations, approvals, and execution paths tied to the facts that justify them. When a premise changes, Velarix retracts unsupported conclusions and blocks stale execution.

## What Velarix Includes

- Go HTTP API for facts, invalidation, explanations, slices, governance, decisions, and execution checks
- symbolic truth-maintenance engine with OR-of-AND justifications and negated dependencies
- The core engine is pure symbolic reasoning with no LLM in the hot path, and it is explicitly not an LLM wrapper.
- query-aware belief slicing with semantic ranking and dependency expansion
- review-gated governance controls for protected facts and mutations
- Python SDK plus OpenAI, LangGraph, LlamaIndex, and CrewAI integration surfaces
- reproducible long-horizon contradiction benchmark harness
- `vlx` CLI for health, slice, review, mutation, compliance export, and benchmark workflows

## Quickstart

Start the API locally:

```bash
export VELARIX_ENV=dev
export VELARIX_API_KEY=dev-admin-key
export VELARIX_BADGER_PATH="$(mktemp -d)"
go run main.go
```

Write facts with the Python SDK:

```python
from velarix import VelarixClient

client = VelarixClient(base_url="http://localhost:8080", api_key="dev-admin-key")
session = client.session("approval-demo")

session.observe("vendor_verified", {"summary": "Vendor 17 passed KYB"})
session.observe("invoice_approved", {"summary": "Invoice inv-1042 is approved"})

session.derive(
    "decision.release_payment",
    [["vendor_verified", "invoice_approved"]],
    {"summary": "Release payment for invoice inv-1042"},
)
```

## Operating Pattern

Velarix belongs at the execution boundary:

1. record the approval facts
2. derive the recommendation
3. create a first-class decision
4. call `execute-check` immediately before the side effect
5. execute only if the decision remains valid

## Truth Semantics

Velarix supports `assertion_kind` scoping on facts (`empirical`, `uncertain`, `hypothetical`, `fictional`) so creative/hypothetical content does not generate false contradiction signals and cannot ground real-world derived conclusions. See [docs/README.md](docs/README.md) and the fact schema in [docs/openapi.yaml](docs/openapi.yaml).

Facts may include `valid_until` (unix ms). After expiry, facts are treated as invalid and an expiry sweep persists `fact_expired` events so downstream dependents are invalidated promptly and reloads reconstruct the same state. See [docs/README.md](docs/README.md).

Velarix also supports org-wide global facts (`/v1/global/facts`) that fan out into active sessions to provide shared ground truths (e.g. verified entities or org-wide reference facts). See [docs/README.md](docs/README.md) and [docs/openapi.yaml](docs/openapi.yaml).

Velarix can also store verification metadata for facts (verified/unverified/rejected) and use org policy to prevent unverified, untrusted, or stale premises from grounding execution-critical conclusions. Admins can update verification via `POST /v1/s/{session_id}/facts/{fact_id}/verify`, and optional webhook automation can apply verification decisions out-of-band. See [docs/README.md](docs/README.md).

## Extraction

Velarix includes a fact extraction pipeline that converts raw LLM output into atomic facts with inferred dependencies, then (optionally) checks for contradictions before assertion.

- Endpoint: `POST /v1/s/{session_id}/extract-and-assert`
- Model: set `VELARIX_EXTRACTOR_MODEL` for extraction calls (separate from `VELARIX_VERIFIER_MODEL`)
- Optional request field: `extraction_config` to enable/disable pipeline stages for benchmarking

Example request body:

```json
{
  "llm_output": "…",
  "session_context": "…",
  "auto_retract_contradictions": false,
  "extraction_config": {
    "EnableSelection": true,
    "EnableDecontextualisation": true,
    "EnableCoverageVerification": false,
    "EnableConsistencyPrecheck": true
  }
}
```

### Tiered Extraction Architecture

Velarix supports three extraction tiers, selectable via `VELARIX_EXTRACTION_TIER` or the `extraction_config.Tier` field:

| Tier | Name | Pipeline | Cost | Latency |
|------|------|----------|------|---------|
| 1 | SRL (default) | spaCy + AllenNLP SRL via local Python service | Zero | ~50ms |
| 2 | Hybrid | SRL first, LLM fallback for low-confidence sentences | Low | ~200ms |
| 3 | Full LLM | Five-stage LLM pipeline (selection → decomposition → coverage → consistency) | Standard | ~3s |

```json
{
  "extraction_config": {
    "Tier": 1,
    "SRLServiceURL": "http://localhost:8090"
  }
}
```

The SRL service runs as a sidecar (`extractor/srl_service/`). Start it with:

```bash
cd extractor/srl_service && pip install -r requirements.txt && python main.py
```

## Integration Surfaces

- Python SDK: session, fact, slice, and decision APIs
- OpenAI adapter: model-facing observation and slice injection
- LangGraph: checkpoint-backed graph state in Velarix
- CrewAI: query-aware belief injection into task descriptions
- LlamaIndex: lightweight retrieval of current valid beliefs

## Documentation

- [Repository Overview](docs/README.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Integration Guide](docs/INTEGRATION_GUIDE.md)
- [Operations](docs/OPERATIONS.md)
- [Security Notes](docs/SECURITY.md)
- [Threat Model](docs/THREAT_MODEL.md)
- [Errors](docs/ERRORS.md)
- [Benchmarking And Deployment](BENCHMARKING_AND_DEPLOYMENT.md)
- [Python SDK](sdks/python/README.md)

## Canonical Examples

- [`demo/approval_integrity.py`](demo/approval_integrity.py)
- [`demo/langgraph_integration.py`](demo/langgraph_integration.py)
- [`demo/crewai_integration.py`](demo/crewai_integration.py)
- [`tests/reproducibility/hallucination_benchmark.py`](tests/reproducibility/hallucination_benchmark.py)

## Production Notes

- production requires `VELARIX_JWT_SECRET`
- browser access in production requires `VELARIX_ALLOWED_ORIGINS`
- production runs on Postgres-backed runtime state
- Badger outside development requires explicit opt-in
- Redis coordination is recommended for multi-instance rate limiting and idempotency

## Design Rule

If a workflow can move money, change access, or create audit exposure, require a fresh `execute-check` before the final action.

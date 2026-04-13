# Velarix: Decision Integrity for AI Agents

![Velarix](https://img.shields.io/badge/Status-Alpha-orange) ![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)

Velarix is the decision-integrity layer for AI-assisted operations.

It keeps recommendations, approvals, and execution paths tied to the facts that justify them. When a premise changes, Velarix retracts unsupported conclusions and blocks stale execution.

## What Velarix Includes

- Go HTTP API for facts, invalidation, explanations, slices, consistency checks, decisions, and execution checks
- symbolic truth-maintenance engine with OR-of-AND justifications and negated dependencies
- query-aware belief slicing with semantic ranking and dependency expansion
- review-gated governance controls for protected facts and mutations
- org-scoped auth, API keys, invitations, notifications, compliance export, and support-ticket surfaces
- Python SDK plus OpenAI, LangChain, LangGraph, LlamaIndex, and CrewAI integration surfaces
- reproducible contradiction benchmarks in both Go and Python
- `vlx` CLI for health, slice, review, mutation, compliance export, and benchmark workflows

## Quickstart

Start the API locally:

```bash
export VELARIX_ENV=dev
export VELARIX_BADGER_PATH="$(mktemp -d)"
go run main.go --lite
```

Write facts with the Python SDK:

```python
from velarix import VelarixClient

client = VelarixClient(base_url="http://localhost:8080")
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

## Integration Surfaces

- Python SDK: session, fact, slice, and decision APIs
- OpenAI adapter: model-facing observation and slice injection
- LangChain: wrapped chat model with Velarix runtime injection
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
- browser access with auth cookies requires `VELARIX_ALLOWED_ORIGINS`
- production runs on Postgres-backed runtime state
- Badger outside development requires explicit opt-in
- Redis coordination is recommended for multi-instance rate limiting and idempotency

## Design Rule

If a workflow can move money, change access, or create audit exposure, require a fresh `execute-check` before the final action.

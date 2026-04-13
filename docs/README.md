# Documentation

Velarix is the decision-integrity service for AI-assisted approvals and operational actions.

The product is built around four guarantees:

- facts are explicit
- support is traceable
- stale conclusions are retractable
- unsafe decisions are blocked before execution

## What Ships In This Repository

- Go API for facts, explanations, slices, governance, consistency checks, and first-class decisions
- symbolic reasoning engine with dependency tracking and negated support
- query-aware belief retrieval for agent runtimes
- org-scoped auth, API keys, invitations, notifications, policies, billing metadata, support tickets, backup export, and compliance export
- Python SDK and runtime helpers
- LangChain, LangGraph, CrewAI, LlamaIndex, and OpenAI integration surfaces
- `vlx` CLI for core operational workflows
- reproducible contradiction benchmarks in Python and Go
- maintained demos for approval integrity and framework integrations
- optional docs-page endpoints backed by repo-local `markdown/*.md` files

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
- Swagger 2 compatibility stubs that point at the maintained OpenAPI document: [`swagger.yaml`](swagger.yaml), [`swagger.json`](swagger.json), [`docs.go`](docs.go)
- Postman collection for the maintained subset: [`postman.json`](postman.json)

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
- The benchmark surfaces are `tests/reproducibility/hallucination_benchmark.py` and the Go `benchmark/` package.

# Documentation

Velarix should be understood as an approval guardrail service for AI-assisted internal operations.

Its job is simple:

- record the facts behind an approval recommendation
- track the dependencies that support the decision
- block execution when those dependencies go stale

The first wedge for this repo is:

- finance ops approval integrity

## What The Repo Ships Today

- Go API for facts, invalidation, history, explanation, and first-class decisions
- Python SDK
- local Badger adapter for development and tests
- shared-state path using Postgres with optional Redis coordination
- one maintained demo: `demo/approval_integrity.py`

## What The Repo Does Not Claim

- a finished finance ops SaaS product
- a generic memory platform for agents
- audited compliance posture
- production-complete object storage, billing, support, or policy workflows

## Key References

- [Architecture](ARCHITECTURE.md)
- [Integration Guide](INTEGRATION_GUIDE.md)
- [Operations](OPERATIONS.md)
- [Security Notes](SECURITY.md)
- [Threat Model](THREAT_MODEL.md)
- [Errors](ERRORS.md)
- OpenAPI document: [`openapi.yaml`](openapi.yaml)

## Read This Repo In Order

1. `README.md`
2. `docs/ARCHITECTURE.md`
3. `api/models/decision.go`
4. `api/decision_contracts.go`
5. `api/server.go`
6. `store/interfaces.go`
7. `store/badger.go`
8. `store/postgres/`
9. `demo/approval_integrity.py`
10. `tests/e2e_test.go`

## Review Notes

- The canonical product flow is in `api/decision_contracts.go`.
- The canonical demo is `demo/approval_integrity.py`.
- The Postgres path is the production direction.
- The Badger path exists for local development and tests.

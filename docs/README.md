# Documentation

Velarix is a decision-integrity service for AI-assisted internal approvals. It records facts, tracks causal dependencies, and invalidates stale reasoning before downstream actions are taken.

## What The Repo Ships Today

- Go API for session facts, invalidation, explanations, history, and exports
- Python and TypeScript SDKs
- local Badger-backed persistence for development and tests
- one maintained demo: `demo/approval_integrity.py`

## What It Does Not Claim Today

- no shipped console application in this repository,
- no audited compliance certification,
- no production shared-store backend,
- no real billing, support, or policy-enforcement workflow.

## Key References

- [Architecture](ARCHITECTURE.md)
- [Security Notes](SECURITY.md)
- [Integration Guide](INTEGRATION_GUIDE.md)
- [Operations](OPERATIONS.md)
- [Errors](ERRORS.md)
- OpenAPI document: [`swagger.yaml`](swagger.yaml)

## Quick Start

```bash
export VELARIX_ENV=dev
export VELARIX_API_KEY=dev-admin-key
go run main.go
```

The API will be available at `http://localhost:8080`.

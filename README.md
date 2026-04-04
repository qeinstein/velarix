# Velarix

Velarix is a Go service for recording approval facts, tracking decision dependencies, and blocking execution when those dependencies go stale.

The codebase is currently optimized around one workflow shape:

- an internal approval or recommendation is created from a set of facts
- one or more upstream facts change
- `execute-check` or `execute` rejects the stale decision
- the API returns machine-readable blockers plus an explanation

## What Is In This Repo

- Go HTTP API for facts, decisions, invalidation, explanations, history, and search
- decision lifecycle routes: create, recompute, execute-check, execute, lineage, why-blocked
- Python and TypeScript SDKs
- Badger-backed local adapter for development and tests
- Postgres-backed shared-state path with optional Redis coordination
- one maintained end-to-end demo: `demo/approval_integrity.py`

## What Is Not In Scope

- a finished finance ops SaaS product
- a complete console application
- full external workflow integrations
- audited compliance posture
- finalized billing or support systems

Some admin and org routes still exist in the API, but the repo should be evaluated on the approval-guardrail workflow, not on those scaffolding surfaces.

## Core Workflow

1. Assert root facts into a session.
2. Derive a decision-supporting fact.
3. Create a first-class decision.
4. Invalidate or change an upstream fact.
5. Call `execute-check` and receive blockers plus an execution token when valid.
6. Call `execute` with a fresh execution token.

If the dependency set has changed, execution is blocked.

## Runtime Model

Current runtime shape:

- Go API server
- per-session in-memory engines used as rebuildable caches
- Badger for local development and tests
- Postgres as the shared system-of-record path
- Redis for idempotency and rate limiting when configured

The Badger backend should be treated as a local adapter, not the long-term production authority.

## Quick Start

Use the local Badger adapter:

```bash
cp .env.example .env
go run main.go
```

Or set the environment explicitly:

```bash
export VELARIX_ENV=dev
export VELARIX_API_KEY=dev-admin-key
export VELARIX_STORE_BACKEND=badger
export VELARIX_BADGER_PATH="$(mktemp -d)"
go run main.go
```

Run the maintained demo from a second terminal:

```bash
pip install -e ./sdks/python
export VELARIX_API_KEY=dev-admin-key
python demo/approval_integrity.py
```

## Tests

Go:

```bash
go test ./... -count=1 -p 1
```

Python SDK integration:

```bash
python3 -m venv .venv-sdk-tests
./.venv-sdk-tests/bin/pip install pytest requests httpx
PYTHONPATH=./sdks/python ./.venv-sdk-tests/bin/python -m pytest sdks/python/tests/test_sdk_integration.py
```

TypeScript SDK integration:

```bash
cd sdks/typescript && npm test
```

## Review Map

Start here if you are reading the code for the first time:

1. `core/fact.go`
2. `core/engine.go`
3. `core/explain.go`
4. `api/models/decision.go`
5. `api/decision_contracts.go`
6. `api/server.go`
7. `store/interfaces.go`
8. `store/badger.go`
9. `store/postgres/`
10. `demo/approval_integrity.py`

## Docs

- [`docs/README.md`](docs/README.md)
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)
- [`docs/INTEGRATION_GUIDE.md`](docs/INTEGRATION_GUIDE.md)
- [`docs/OPERATIONS.md`](docs/OPERATIONS.md)
- [`docs/SECURITY.md`](docs/SECURITY.md)
- [`docs/THREAT_MODEL.md`](docs/THREAT_MODEL.md)
- [`docs/ERRORS.md`](docs/ERRORS.md)

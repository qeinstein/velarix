# Velarix

Velarix prevents stale AI-assisted approval decisions from being executed after upstream facts change.

This repository currently ships:
- a Go API for recording facts, derivations, invalidations, history, explanations, and exports,
- first-class decision creation, execute-check, execute, lineage, and why-blocked APIs,
- Python and TypeScript SDKs that talk to that API,
- backend selection for local Badger or shared Postgres with optional Redis coordination,
- one canonical demo for approval integrity.

This repository does not currently ship:
- a production console application,
- object-storage-backed export artifacts,
- real billing, support, or policy-enforcement workflows.

## Product Wedge

The V1 wedge is decision integrity for AI-assisted internal approvals.

In practice that means:
- an agent or operator records the facts behind an approval recommendation,
- downstream reasoning depends on those facts,
- if an upstream fact changes, stale derived reasoning is invalidated,
- operators can inspect the history and explanation trail before acting.

## Current Architecture

Today the service runs as:
- a Go HTTP API layer,
- in-memory reasoning engines loaded lazily per session,
- session-version-based cache refresh for multi-instance safety,
- either a local Badger adapter or a shared Postgres backend,
- optional Redis-backed idempotency and rate limiting.

Target V1 architecture is shared-state:
- Postgres for durable system-of-record data,
- Redis for idempotency, rate limiting, and coordination,
- object storage for large artifacts and exports,
- rebuildable in-memory engine caches as an optimization only.

The Badger path in this repo should be treated as a local adapter, not the final production authority.

## Canonical Demo

The canonical demo path for this repo is:

```bash
cp .env.example .env
export VELARIX_ENV=dev
export VELARIX_API_KEY=dev-admin-key
export VELARIX_BADGER_PATH="$(mktemp -d)"
go run main.go
```

In a second terminal:

```bash
pip install -e ./sdks/python
export VELARIX_API_KEY=dev-admin-key
python demo/approval_integrity.py
```

That demo uses only the shipped public Python SDK methods and shows:
- facts recorded into a session,
- a first-class decision created from a derived fact,
- an upstream fact invalidated,
- execute-check returning `executable=false`,
- the blocked execution response and why-blocked explanation payload.

To boot against shared storage instead of Badger:

```bash
cp .env.example .env
export VELARIX_ENV=dev
export VELARIX_STORE_BACKEND=postgres
export VELARIX_POSTGRES_DSN=postgres://user:pass@localhost:5432/velarix?sslmode=disable
export VELARIX_REDIS_URL=redis://localhost:6379/0
go run main.go
```

The committed env template is [`.env.example`](/home/fluxx/Workspace/casualdb/.env.example). It now includes:
- local Badger variables,
- shared Postgres/Redis variables,
- dev versus production auth/runtime knobs,
- the current optional tuning variables used by the server.

## Repository Notes

- `demo/approval_integrity.py` is the maintained demo for the V1 wedge.
- `docs/openapi.yaml` is the stable API contract entrypoint served at `/docs/openapi.yaml`.
- The other org/admin surfaces in the API are existing scaffolding and should not be read as completed enterprise workflows.
- Password reset is intentionally dev-only until a real delivery path exists.
- Invitation tokens are returned once on create and redacted from list/read responses.
- Export files include verification hashes, but the repo does not claim audited compliance posture.

## Development

Start by copying the env template:

```bash
cp .env.example .env
```

For local Badger development, the minimum useful env is:

```bash
export VELARIX_ENV=dev
export VELARIX_API_KEY=dev-admin-key
export VELARIX_STORE_BACKEND=badger
export VELARIX_BADGER_PATH="$(mktemp -d)"
go run main.go
```

For shared-store development, set:

```bash
export VELARIX_ENV=dev
export VELARIX_STORE_BACKEND=postgres
export VELARIX_POSTGRES_DSN=postgres://user:pass@localhost:5432/velarix?sslmode=disable
export VELARIX_REDIS_URL=redis://localhost:6379/0
go run main.go
```

Run the Go tests with:

```bash
go test ./... -count=1 -p 1
```

Run the Python SDK integration test with:

```bash
python3 -m venv .venv-sdk-tests
./.venv-sdk-tests/bin/pip install pytest requests httpx
PYTHONPATH=./sdks/python ./.venv-sdk-tests/bin/python -m pytest sdks/python/tests/test_sdk_integration.py
```

Run the TypeScript SDK integration test with:

```bash
cd sdks/typescript && npm test
```

For local development, `VELARIX_ENV=dev` disables the production-only JWT secret requirement, allows Badger without an encryption key, and enables dev-only password reset token issuance in the response body instead of logs. Outside `dev`, set `VELARIX_JWT_SECRET`, and if you still use Badger, set a valid `VELARIX_ENCRYPTION_KEY`.

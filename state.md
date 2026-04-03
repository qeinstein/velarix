# State

Last updated: 2026-04-03 after Milestones 3, 5, 6, and 7 implementation pass

## Current Goal

Close the remaining V1 gaps around shared state, public contract alignment, security hardening, and release-gate coverage.

## Completed Before This Pass

- Milestone 0 credibility cleanup completed
- Store interfaces introduced so `api.Server` no longer depends on `*store.BadgerStore`
- Badger reframed as the local adapter in `main.go` and `store/interfaces.go`
- Existing Go test suite passing at the end of Milestone 1

## Completed In This Pass

- Milestone 3 shared-state backend:
  - added `store/postgres/` with SQL migrations and a Postgres-backed `RuntimeStore`
  - added `store/redis/` for shared idempotency and rate-limit state
  - added `store/runtime_composite.go` for Postgres+Redis runtime composition
  - wired backend selection in `main.go` via `VELARIX_STORE_BACKEND`, `VELARIX_POSTGRES_DSN`, `VELARIX_REDIS_URL` / `VELARIX_REDIS_ADDR`, and `VELARIX_BADGER_PATH`
  - removed eager boot-time `ReplayAll`; session engines now rebuild lazily from persisted snapshots/history and use session-version cache refresh
- Milestone 5 SDK/OpenAPI/demo alignment:
  - Python SDK session objects now expose decision create/list/get/recompute/execute-check/execute/why-blocked flows
  - TypeScript SDK session objects now expose the same decision workflow surface
  - `docs/openapi.yaml` now documents the stable decision-integrity workflow routes
  - `/docs/openapi.yaml` now serves the stable OpenAPI file
  - `demo/approval_integrity.py` now uses the public decision SDK methods end to end
- Milestone 6 security and operational hardening:
  - password reset tokens are no longer logged
  - password reset is explicit dev-only until a real delivery path exists
  - invitation create/list/revoke/accept flows now redact tokens after issuance and audit lifecycle events into org activity
  - retention enforcement now has a background sweep ticker in `api.Server`
- Milestone 7 tests and release gate:
  - added decision contract coverage
  - added multi-instance cache-refresh coverage
  - added idempotency replay coverage
  - added backpressure header coverage
  - added invitation secrecy and retention-enforcement coverage
  - bounded the default stress test so the default test suite remains runnable in constrained environments

## Notes For Resume

- Shared-state runtime now exists, but the default local path is still Badger unless `VELARIX_STORE_BACKEND=postgres` is set.
- The Postgres store includes fallback idempotency/rate-limit tables; Redis overrides those when configured.
- The live Go release-gate suite is passing with `go test ./... -count=1 -p 1`.
- Python SDK live integration coverage is passing via a local virtualenv-based pytest run.
- TypeScript SDK live integration coverage is passing via `npm test`.
- The remaining practical gap is environment-backed coverage for real external Postgres and Redis instances; the code paths exist and compile, but the current test run did not stand up external services.

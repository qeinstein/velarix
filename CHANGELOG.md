# Changelog

All notable changes to this project are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
- Repository hygiene: `.gitignore` extended to cover compiled binaries, build outputs (`web/.next/`), log files, Python artifacts (`*.egg-info/`), benchmark generated outputs, and `node_modules/`
- `.env.example` extended with all env vars read by the server: `VELARIX_DECISION_TOKEN_SECRET`, `VELARIX_BADGER_PATH`, `VELARIX_DISABLE_REDIS`, `VELARIX_MAX_CONCURRENT_WRITES`, `VELARIX_METRICS_ALLOWED_CIDR`, `VELARIX_OPENAI_BASE_URL`, `OPENAI_API_KEY`, `VELARIX_VERIFIER_MODEL`, `VELARIX_VERIFIER_MAX_PAIRS`, `VELARIX_RETENTION_SWEEP_INTERVAL_MINUTES`, `VELARIX_BASE_URL`, `VELARIX_SMTP_*`
- `CHANGELOG.md` created
- `CONTRIBUTING.md` updated with local setup, test, benchmark, and branch convention instructions
- Documentation accuracy pass across README, operational docs, benchmark notes, API specs, demo docs, env var reference, Go godoc, and Python SDK docstrings

### Removed
- Compiled Go binaries (`velarix`, `vlx`, `tests.test`) removed from git tracking
- `web/.next/` build cache removed from git tracking
- `node_modules/` removed from git tracking
- `sdks/python/velarix.egg-info/` removed from git tracking
- `benchmark/baseline_core.txt`, `benchmark/baseline_internal.txt`, `benchmark/coverage.txt`, `benchmark/coverage_full.out`, `benchmark/govulncheck.txt` removed from git tracking
- `web/.env.local`, `web/dev.log` removed from git tracking

---

## [0.1.0-benchmark] — 2026-04-12

### Added — Performance sprint (commit 81a9b40)
- 20 performance, accuracy, and efficiency fixes across the core reasoning engine
- Improved cycle detection, dominator computation, and dependency graph traversal
- Composite runtime store with Redis coordination path for idempotency and rate limiting
- Benchmark harness (`benchmark/harness.go`, `benchmark/datasets.go`) for reproducible contradiction and decision integrity testing
- Threshold analysis test suite (`benchmark/threshold_analysis_test.go`)
- `VERSION` file set to `0.1.0-benchmark`

### Added — Security sprint (commit 0cbfd3a)
- 17 security findings remediated across auth, tenant isolation, secrets handling, and runtime hardening
- JWT secret minimum-length enforcement (32 bytes) at startup
- `VELARIX_DECISION_TOKEN_SECRET` for signing decision execution tokens independently of the console JWT
- CORS fails closed — `VELARIX_ALLOWED_ORIGINS` required in production
- TLS configuration via `VELARIX_TLS_CERT` / `VELARIX_TLS_KEY` with plaintext warning when unset
- Rate limiting and request size limits (4 MB body cap, 1 MB header cap)
- Production Badger fallback blocked by default; requires `VELARIX_ALLOW_BADGER_PROD=true`
- `VELARIX_TRUST_PROXY_HEADERS` controls `X-Forwarded-For` / `X-Real-Ip` trust
- Bootstrap admin key disabled by default (`VELARIX_ENABLE_BOOTSTRAP_ADMIN_KEY=false`)
- Cross-tenant isolation hardening in session and decision handlers
- `redactDSN` helper prevents DSN credentials from appearing in structured logs
- `VELARIX_METRICS_ALLOWED_CIDR` to restrict Prometheus metrics endpoint access

### Added — Auth and SDK sprint (commit ff6d51a)
- Velarix-native authentication required by default (replaces open-by-default mode)
- `handleRegister`, `handleLogin`, `handleLogout`, `handleResetRequest`, `handleResetConfirm` auth routes
- `handleMe`, `handleChangePassword`, `handleGetOnboarding`, `handleUpdateOnboarding` account routes
- Session management routes: `POST /v1/org/sessions`, `PATCH /v1/org/sessions/{id}`, `DELETE /v1/org/sessions/{id}`
- Python SDK `VelarixSession.extract_and_assert()` method for LLM-assisted fact extraction
- Python SDK `VelarixClient.create_session()` for explicit session creation
- Python SDK `VelarixSession.delete()` for session teardown
- OpenAI adapter authentication fix — API key is now passed correctly

### Added — Extractor
- `extractor/extractor.go` — structured fact extraction from free-text using an LLM backend
- `extractor/extractor_test.go` — unit tests for extraction pipeline
- `POST /v1/s/{session_id}/extract-and-assert` API endpoint

### Added — Core engine
- Symbolic truth-maintenance engine with OR-of-AND justifications and negated dependencies
- Query-aware belief slicing with semantic ranking and dependency expansion
- Review-gated governance controls for protected facts and mutations
- Consistency verifier with LLM-backed contradiction detection (`VELARIX_VERIFIER_MODEL`)
- Decision lifecycle: create, recompute, execute-check, execute, lineage, why-blocked
- Reasoning chain recording and verification
- Export and export-job endpoints for compliance and audit workflows
- SSE event stream (`GET /v1/s/{session_id}/events`)
- Semantic search backed by pgvector embeddings

### Added — Python SDK
- `VelarixClient` and `VelarixSession` synchronous clients
- `AsyncVelarixClient` and `AsyncVelarixSession` async clients
- `VelarixRuntime` for model-facing observation and slice injection
- LangGraph, CrewAI, LlamaIndex, and LangChain integration surfaces
- OpenAI adapter (`velarix.adapters.openai`)
- MCP server (`velarix.mcp_server`)
- `vlx` CLI for health, slice, review, mutation, compliance export, and benchmark workflows

### Added — Infrastructure
- `Dockerfile` for containerised deployment
- Control-plane stubs: billing service, Terraform infra, tenant provisioning script
- GitHub Actions publish workflow (`.github/workflows/publish.yml`)
- OpenAPI spec (`docs/openapi.yaml`), Postman collection (`docs/postman.json`), Swagger (`docs/swagger.yaml`, `docs/swagger.json`)
- `docs/ARCHITECTURE.md`, `docs/INTEGRATION_GUIDE.md`, `docs/OPERATIONS.md`, `docs/SECURITY.md`, `docs/THREAT_MODEL.md`, `docs/ERRORS.md`
- `BENCHMARKING_AND_DEPLOYMENT.md`

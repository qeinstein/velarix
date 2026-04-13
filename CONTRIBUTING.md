# Contributing to Velarix

Velarix accepts contributions across the reasoning engine, API, SDKs, integrations, benchmarks, and docs.

## Development Setup

### Prerequisites

- Go 1.26+
- Python 3.10+
- (Optional) PostgreSQL 15+ with pgvector for production-mode store
- (Optional) Redis 7+ for multi-instance rate limiting and idempotency

### Local server (Lite mode — no auth, local BadgerDB)

```bash
cp .env.example .env
export VELARIX_ENV=dev
export VELARIX_LITE=true
export VELARIX_BADGER_PATH="$(mktemp -d)"
go run main.go
```

The server starts at `http://localhost:8080`.

### Local server (Full mode — JWT auth, Postgres)

```bash
cp .env.example .env
# Edit .env — set VELARIX_JWT_SECRET (min 32 chars), VELARIX_POSTGRES_DSN, etc.
go run main.go
```

### Python SDK (development install)

```bash
pip install -e ./sdks/python
# With optional extras:
pip install -e './sdks/python[langgraph,langchain,crewai,llamaindex]'
```

## Running Tests

### Go tests

```bash
# All tests (requires VELARIX_ENV=dev or test)
VELARIX_ENV=dev go test ./...

# A single package
VELARIX_ENV=dev go test ./core/...

# With race detector
VELARIX_ENV=dev go test -race ./...
```

### Python SDK tests

```bash
pip install -e './sdks/python[dev]'
cd sdks/python
pytest tests/
```

## Running the Benchmark Harness

The benchmark harness exercises contradiction detection, decision integrity, and performance baselines. It requires a running Velarix server.

```bash
# Start the server in dev/lite mode first, then:
VELARIX_ENV=dev go test ./benchmark/ -run TestBenchmark -v -timeout 10m
```

See `BENCHMARKING_AND_DEPLOYMENT.md` for full benchmark configuration and threshold analysis.

## Running the vlx CLI

```bash
go build -o vlx ./cmd/vlx
./vlx --help
```

## Branch and PR Convention

1. Create a branch from `main` — name it after the surface you are changing (e.g. `core/cycle-fix`, `sdk/async-client`, `docs/env-vars`)
2. Make the change with tests
3. Open a pull request with a concise description of the behaviour change and the affected product surface
4. Keep PRs scoped — one logical change per PR

## Code Standards

### Go

- Use idiomatic Go formatting (`gofmt`)
- Include tests with reasoning, API, or persistence changes
- Call out complexity impact when changing graph or invalidation logic
- Do not disable linter rules without explanation

### Python SDK

- Keep type hints accurate
- Keep integration examples aligned with shipped SDK surfaces

### Documentation

- Keep public docs product-facing
- Keep operational claims aligned with the code
- Update `.env.example` when adding a new env var
- Update `CHANGELOG.md` under `[Unreleased]` for every user-visible change

## Issues

Use GitHub issues for bugs, regressions, and feature requests.

Include reproduction steps whenever possible.

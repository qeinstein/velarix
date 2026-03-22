# Backend Management Overview

Velarix uses a Go-based backend with a focus on deterministic reasoning as an epistemic state layer for AI agents.

## 🛠️ Core Technology Stack
- **Language**: Go 1.25.0
- **Database**: [BadgerDB v4](https://github.com/dgraph-io/badger) (Embedded Key-Value store)
- **Persistence Layer**: Custom `store` package wrapping BadgerDB with journaled writes (`WithSyncWrites(true)`) and snapshots.
- **Server Framework**: Standard `net/http` package with custom routing in `api/server.go`.

## 📂 Project Structure
- `api/`: HTTP handlers, JWT authentication, and request middleware.
- `core/`: Core engine logic (Fact assertion, causal invalidation, AND/OR belief propagation).
- `store/`: BadgerDB management, journal replay, and snapshots.
- `sdks/`: Native SDK implementations (Python, TypeScript) and OpenAI adapters.

## 🔒 Security & Compliance
- **Authentication**: JWT-based using a global `VELARIX_API_KEY` or per-organization tokens.
- **Tenant Isolation**: Mandatory `OrgID` filtering on all session-related operations.
- **Encryption**: AES-256 encryption at rest (managed via BadgerDB). Mandatory in production (`VELARIX_ENV != dev`).
- **Audit Trails**: SOC2-compliant logging in CSV/PDF format, exportable via API.
- **Rate Limiting**: Request quotas persisted in BadgerDB.

## ⚙️ Operations & Lifecycle
- **Build**: Managed via `Dockerfile` using a multi-stage Go build.
- **Startup**: "Hybrid Boot" reads the latest snapshot and replays trailing journal entries for consistency.
- **Storage Strategy**: Single-node state layer (BadgerDB is embedded). Horizontal scaling requires sticky sessions or volume sharing (though the latter is discouraged for Badger).
- **Backup & Restore**: Native support for full DB backups (`BadgerStore.Backup/Restore`).
- **Maintenance**:
  - **BadgerDB GC**: Automatic background garbage collection (runs every 30 mins).
  - **Eviction**: Background ticker for session cleanup.
  - **Backups**: Periodically managed via internal tracers.
- **Observability**:
  - **Logging**: Structured JSON logging via `slog`.
  - **Metrics**: Prometheus metrics available at `/metrics`.
  - **Tracing**: OpenTelemetry integration via `api.InitTracer()`.

## 📋 Schema Management
- **Definition**: JSON Schemas are defined per-session.
- **Persistence**: Schemas are stored in the session configuration (`s:{id}:c`) within BadgerDB.
- **Enforcement**:
  - **Strict Mode**: Rejects facts that don't match the schema (400 Bad Request).
  - **Warn Mode**: Accepts facts but tags them with `validation_errors` for downstream filtering.
- **Validation Engine**: Powered by `gojsonschema`.

## 🚀 Key Management Endpoints
- `/health`: Basic health check.
- `/v1/admin/health`: Detailed disk usage and session statistics (Admin required).
- `/v1/audit/export`: SOC2 compliance log exports.
- `/metrics`: Prometheus metrics ingestion.

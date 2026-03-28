# Velarix: The Epistemic State Layer for AI Agents

Velarix is a production-hardened belief-tracking engine designed for AI agents operating in regulated, high-stakes environments. It replaces logically flat memory with a **Stateful Logical Graph** that enforces reasoning integrity by bridging probabilistic LLM outputs with deterministic reasoning.

## Project Overview

- **Core Mission**: Provide a "deterministic conscience" for autonomous agents.
- **Main Technologies**:
  - **Backend**: Go 1.26 (BadgerDB v4, JWT, Prometheus, OpenTelemetry, Swag, gojsonschema).
  - **SDKs**: Python (OpenAI, LangGraph, LlamaIndex integrations), TypeScript.
  - **Persistence**: Hybrid Boot (Binary Snapshots + Journal Replay) for sub-second recovery.
  - **Security**: Mandatory AES-256 encryption at rest, JWT authentication, and strict Tenant Isolation (`OrgID`).

## Architecture

Velarix treats agent memory as a Directed Acyclic Graph (DAG) of **Facts** and **Justifications**.

1. **Epistemic Kernel**: Manages belief propagation and causal invalidation using **Dominator Tree** theory for $O(1)$ pruning of deep dependency chains.
2. **Durable Persistence**: Uses BadgerDB with synchronous IO to guarantee data survival across system crashes.
3. **Interceptor Layer**: Adapters for LLM frameworks to automate fact extraction and provenance tracking.
4. **Observability**: Integrated Prometheus metrics and OpenTelemetry tracing for real-time impact analysis.

## Building and Running

### Prerequisites
- Go 1.26+
- Python 3.9+ (for SDK)
- Node.js (for Console)

### Local Development
1. **Configure Environment**:
   ```bash
   cp .env.example .env
   # Set VELARIX_ENCRYPTION_KEY (32 bytes) and VELARIX_JWT_SECRET
   export VELARIX_ENV="dev" 
   ```
2. **Start the Kernel**:
   ```bash
   go run main.go
   ```
3. **Run Tests**:
   ```bash
   go test -v ./tests/...
   ```

### Production Deployment
- **Docker**: `docker build -t velarix .`
- **Security**: `VELARIX_ENCRYPTION_KEY` and `VELARIX_JWT_SECRET` are **REQUIRED** unless `VELARIX_ENV=dev`.

## SDKs & Integrations

- **Python SDK**: Located in `sdks/python/`. Supports `pip install -e .`.
  - Extras: `[langgraph]`, `[llamaindex]`.
- **TypeScript SDK**: Located in `sdks/typescript/`.
- **API Reference**: Available at `http://localhost:8080/docs/openapi.yaml` when running.

## Development Conventions

- **State Integrity**: All state changes must be journaled and attributable to a verified actor.
- **Idempotency**: Fact assertions (`AssertFact`) are idempotent based on content.
- **Validation**: Strict JSON Schema enforcement is supported per-session.
- **Causal Invalidation**: When a root fact is retracted, all downstream facts exclusively depending on it are automatically invalidated.

## Directory Structure

- `api/`: REST API handlers, middleware, and server logic.
- `core/`: The Epistemic Kernel (Engine, Facts, Justifications, Dominator logic).
- `store/`: Persistence layer (BadgerDB implementation and Journaling).
- `sdks/`: Native client libraries for Python and TypeScript.
- `docs/`: Comprehensive technical documentation and architecture deep-dives.
- `tests/`: End-to-end, stress, and core logic verification tests.

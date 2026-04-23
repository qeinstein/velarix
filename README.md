# Velarix

A Truth Maintenance System for AI agent decision integrity.

Built over six months by a student trying to solve a real problem in how AI agents make decisions. This is a research experiment, not a production system. It hits a real wall — documented honestly below.

---

## The Problem

When an AI agent decides to take an action, it reasons from a set of beliefs it formed earlier. By the time it executes, some of those beliefs may no longer be true. The agent acts anyway — not because it hallucinated, but because nothing told it to stop.

Velarix was built to be the thing that tells it to stop.

---

## What Was Built

### Core Engine (`/core/`)

A Truth Maintenance System in Go implementing OR-of-AND justification sets. Facts have explicit dependencies. When a fact is retracted, every conclusion that depended on it is automatically invalidated and propagation cascades through the graph.

- Cycle detection
- Negated dependencies (fact X is valid only when fact Y is *not* valid)
- Temporal expiry (`valid_until` timestamps)
- Dominator tree for graph traversal optimization
- Epistemic scoping: `empirical`, `uncertain`, `hypothetical`, `fictional` — hypothetical reasoning does not contaminate real-world conclusions

### Decision Gate (`/api/decision_contracts.go`)

The mechanism closest to working as intended:

1. Assert the facts that must be true for an action to be valid
2. Create a decision referencing those facts
3. Call `execute-check` immediately before executing
4. Velarix snapshots the current validity state of every supporting fact
5. If all facts are valid: issues a short-lived JWT execution token (30s TTL, bound to the session version)
6. If any fact has been retracted or expired: execution is blocked

The token is the audit record. It encodes exactly what was valid, at what version of the world state, at what timestamp.

### Audit Trail

Every assertion, retraction, and decision check is written to an append-only journal. Each entry is SHA-256 chained to the previous one — tamper-evident by construction.

### Extraction Pipeline (`/extractor/srl_service/`)

A Python sidecar (FastAPI) that attempts to convert raw text into structured facts using spaCy dependency parsing, coreferee coreference resolution, and optional GLiNER NER.

**This is where the system breaks.** See below.

### Store Backends

- **BadgerDB** — embedded, for development and tests
- **Postgres** — production, with pgx/v5 connection pooling

### Python SDK + Integrations (`/sdks/python/`)

Sync and async clients. Integrations for LangChain, LangGraph, CrewAI, LlamaIndex, OpenAI function calling.

### Web Dashboard (`/web/`)

Next.js 14. Session management, usage metrics, API key management, team/invite controls.

---

## The Wall

The engine is correct. The extraction pipeline is where this breaks down.

For the system to work, an LLM agent needs to correctly populate the belief store — assert the right facts with the right justification relationships, and retract beliefs when they become invalid. In practice, LLMs do not do this reliably. They are probabilistic text generators, not structured knowledge managers. The extraction pipeline that was supposed to convert LLM output into structured facts works on simple cases and fails on complex ones: wrong entity links, incorrect causal relationships, missed dependencies.

This is not a bug that can be patched. It is a mismatch between what the TMS requires (structured, complete, correctly justified belief graphs) and what current LLMs can reliably produce. That mismatch is an open problem.

---

## Known Architectural Issues

Documented here so anyone continuing this work doesn't hit the same walls:

**1. O(N²) consistency check**
`core/consistency.go:CheckConsistency` compares every fact pair. At scale this is unusable. Needs a semantic index to limit comparisons to approximate nearest neighbours.

**2. gob snapshot serialization**
`core/engine.go:ToSnapshot` uses `encoding/gob`. Any struct field change breaks existing snapshots. Needs versioned JSON or Protobuf before any customer data accumulates.

**3. In-memory session architecture**
All sessions are hydrated into memory per process. Not horizontally scalable without session affinity or a distributed engine design.

**4. GetStatus acquires a write lock**
`DirtyDominators` triggers a full dominator recompute under a write lock on every status read after any mutation. High write rates will stall reads.

---

## Running It

### Go API

```bash
export VELARIX_ENV=dev
export VELARIX_API_KEY=dev-admin-key
export VELARIX_BADGER_PATH="$(mktemp -d)"
go run main.go
```

### Tests

```bash
go test ./... -v
```

### Python Extraction Sidecar (optional)

```bash
cd extractor/srl_service
pip install -r requirements.txt
python -m spacy download en_core_web_sm
python main.py
```

### Python SDK

```bash
pip install -e ./sdks/python
```

### Web App

```bash
cd web
npm install
npm run dev
```

---

## Environment Variables

| Variable | Notes |
|---|---|
| `VELARIX_ENV` | `dev` / `test` / `prod` |
| `VELARIX_API_KEY` | Bootstrap admin bearer token |
| `VELARIX_BADGER_PATH` | Local embedded DB path |
| `VELARIX_POSTGRES_DSN` | Required for Postgres backend |
| `VELARIX_JWT_SECRET` | Required in production |
| `VELARIX_SRL_SERVICE_URL` | Delta sidecar (default: `http://localhost:8090`) |

See `.env.example` for the full list.

---

## Status

This project is not maintained. The codebase is shared as a reference for anyone working on belief revision, decision integrity, or non-monotonic reasoning in agentic AI systems.

The core TMS engine and execution token mechanism work correctly and are worth reading if you are building in this space. The extraction pipeline and the broader product framing do not.

---

## License

Apache 2.0

---

github.com/qeinstein | github.com/qeinstein/velarix

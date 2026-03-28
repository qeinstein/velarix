# Velarix — Epistemic Middleware for AI Agents

Velarix is an **epistemic middleware layer / Truth Maintenance System (TMS)** for AI agents: infrastructure that sits between an application and its agents, tracking every belief, decision, and causal dependency in real time.

Where traditional agent memory stores information and retrieves it by similarity, Velarix stores information **by causality** — every fact knows why it exists, what depends on it, and what should collapse when it becomes false.

Velarix does **not** “stop LLMs from hallucinating” at the generation level. Instead, it controls what the agent is allowed to **believe and act on**. Hallucinations can occur in text, but they don’t become durable beliefs, tool actions, or audit liabilities.

Initial beachhead: **Decision Records via a tool gateway** (AML/KYC is a strong first ICP because the audit question is constant). Velarix is not limited to finance — the same middleware applies to healthcare, insurance, legal, and other regulated workflows.

## Why Velarix?

Traditional “memory” approaches fail in regulated agent systems because they lack a causal model:
- **Causal collapse**: when a foundational belief changes, downstream beliefs and decisions invalidate deterministically.
- **Decision records**: every tool call / policy decision / final outcome can be recorded, replayed, and exported.
- **Audit integrity**: exports include a tamper-evident session journal chain head for integrity verification.
- **Production reliability**: retries are safe via `Idempotency-Key` and scoped API keys limit blast radius.

## The One-Line Swap (Adapters)

The core promise of Velarix is that you don't have to rewrite your agent logic to get epistemic memory. Our adapters provide a drop-in replacement for standard clients.

```python
# Before: from openai import OpenAI
from velarix.adapters.openai import OpenAI

# Initialize once with a session ID
client = OpenAI(velarix_session_id="patient_case_001")

# Everything else remains standard OpenAI code. 
# Velarix automatically injects context and extracts new facts.
response = client.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "The patient has a history of penicillin allergy."}]
)
```

## “Why did you do that?” — Decision Records + Explanations

Velarix supports both:
- **Decision Records**: structured events you write as the agent executes (tool calls, policy decisions, final decisions).
- **Explanations**: deterministic causal chains derived from the belief graph (why/impact).

Decision Records are stored as session history events (`type=decision_record`) and can be exported as audit packs.

### Key Features
- **Deterministic causal chains**: re-trace exactly which facts justified a belief.
- **Impact analysis**: simulate “what collapses if this premise becomes false?”
- **Decision Records**: record tool calls/results and final decisions for audit trails.
- **Exports**: CSV/PDF exports include integrity metadata (journal chain head).

## Quick Start (Under 10 Minutes)

> For the wedge workflow quickstart, see `markdown/AML_KYC_QUICKSTART.md` and `markdown/DECISION_RECORDS.md`.

### 1. Start the Velarix Kernel
Run the server locally using Docker or Go.
```bash
# Set a mandatory encryption key for production-grade security
export VELARIX_ENCRYPTION_KEY="your-32-byte-secure-key-here"
export VELARIX_ENV="dev" # Use dev mode to bypass strict checks for local testing
go run main.go
```
The kernel is now ready at `http://localhost:8080` and serves versioned routes under `/v1`.

### 2. Connect with the Python SDK
Install the SDK and initialize a session.
```python
from velarix import VelarixClient, VelarixGateway

client = VelarixClient(base_url="http://localhost:8080", api_key="your_vx_key")
session = client.session("s_demo")
gateway = VelarixGateway(session)

# Record a tool call as a Decision Record
gateway.call_tool("watchlist.search", {"name": "Ada Lovelace"}, lambda i: {"matches": []})

# Assert a belief (Root Fact)
session.observe("customer.identity_verified", payload={"method": "kyc_vendor"})

# Derive a belief (Child Fact) with causal justification
session.derive(
    "customer.low_risk",
    justifications=[["customer.identity_verified"]],
    payload={"score": 0.12}
)

# Retract the premise: downstream collapses deterministically
session.invalidate("customer.identity_verified")
```

### 3. Launch the Control Plane
Visualize the causal graph and audit reasoning in real-time.
```bash
cd console
npm install
npm run dev
```
Open `http://localhost:5173` to browse sessions, decision records, exports, and the causal graph.

## 🧩 Integration Patterns

Velarix supports multiple levels of integration for AI agents, from simple drop-in replacements to complex reasoning co-processors:
- **Level 1: OpenAI Adapter** (Zero-code migration)
- **Level 2: Epistemic RAG** (Context injection of valid slice)
- **Level 3: Manual Fact Management** (Causal chains)
- **Level 4: Decision Records Gateway** (tool-call capture + audit packs)
- **Level 5: Compliance & Explainability** (exports, why/impact)
- **Level 6: Embedded Co-processor** (sidecar mode)

For full code examples and implementation details, see `markdown/INTEGRATION_PATTERNS.md`.

## Hardened Architecture

Velarix is built for production reliability:
- **Tenant Isolation**: Strict `OrgID` enforcement on every API handler prevents cross-tenant data leaks.
- **Hybrid Boot**: Combines binary snapshots with journal replays for sub-second startup of massive sessions.
- **Durable Journaling**: Every write is synchronized to disk (`Sync()`) to survive system crashes.
- **Actor Attribution & Admin Audit**: Every belief revision tracks the specific API key or User; admin actions (keys, backups, restores, config) are journaled with actor_id and timestamp.
- **Rate Limits with Persistence**: Per-key quotas are stored in BadgerDB and enforced across restarts.
- **Scoped API keys**: `read|write|export|admin` scopes limit access; `auditor` role supports read/export.
- **Hashed API keys (shown once)**: New keys are not stored in plaintext; UI displays redacted prefixes/last4.
- **Access logs + retention**: Org access logs are captured server-side (`GET /v1/org/access-logs`) with retention configured via `/v1/org/settings`.
- **Idempotent writes**: safe retries via `Idempotency-Key`.
- **Tamper evidence**: per-session journal chain head is included in exports.
- **Slice Controls**: `GET /v1/s/{id}/slice` honors `max_facts` to bound prompt size.

## Project Structure

- `/api`: Versioned REST API (`/v1`) with structured JSON logging.
- `/core`: The Epistemic Engine (Dominator Trees & Causal Logic).
- `/store`: Hardened BadgerDB v4 storage with AES-256 encryption.
- `/sdks`: Native Python and TypeScript clients with resource-efficient connection pooling.
- `/console`: React-based Control Plane with world-class motion design and graph visualization.
- `/docs`: OpenAPI/Swagger artifacts; runtime serves docs from `/docs/swagger.yaml`.

---
*Velarix: deterministic truth maintenance for production agents.*

# Velarix: The Epistemic State Layer for Healthcare AI

Velarix is a production-hardened belief-tracking engine designed for AI agents operating in regulated, high-stakes environments. It replaces logically flat memory (vector DBs) with a **Stateful Logical Graph** that enforces reasoning integrity.

In healthcare, agents must not only "remember" context but also **invalidate** state when policy, consent, or lab data changes. Velarix ensures that every agent decision is auditable, deterministic, and cryptographically attributed to a verified actor.

## Why Velarix?

Traditional vector databases fail in healthcare because they lack a causal model:
- **Causal Collapse**: If a patient withdraws HIPAA consent, every downstream belief dependent on that consent must be purged instantly. Velarix handles this with $O(1)$ pruning.
- **Audit Provenance**: Every state change is logged with an `actor_id` and timestamp, exportable as SHA-256 verified SOC2/HIPAA compliance reports.
- **Deterministic Reliability**: Logic-safe propagation ensures agents never act on stale or retracted medical premises.

## The One-Line Swap

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
```

## "Why did you do that?" — Audit-Ready Explanations

Velarix includes a first-class **Reasoning Explanation** feature. When an agent makes a decision based on its belief state, you can ask it to justify that decision. 

Through the `explain_reasoning` tool, the LLM fetches a deterministic causal chain from Velarix, ensuring it never "hallucinates" a justification.

### Key Features
- **Deterministic Causal Chains**: Re-trace exactly which root facts led to a derived belief.
- **Confidence Weighting**: Every node in the explanation includes a confidence score (0.0 - 1.0).
- **Historical Replay**: Ask "Why was this true at 10:00 AM?" to see the state at a specific timestamp.
- **Counterfactual "What If" Analysis**: Predict the impact of removing a specific fact before taking action.
- **Immutability**: All explanations are hashed with SHA-256 and stored in a tamper-evident audit log.

### Using the Tool (Python SDK)

```python
# The explain_reasoning tool is automatically registered with the OpenAI adapter.
# When the user asks "Why did you say that?", the LLM will call the tool.

response = client.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Why did you recommend antibiotics?"}],
    velarix_session="patient_123"
)
```

## Quick Start (Under 10 Minutes)

> **Looking for deep integration details?** See the [Developer Onboarding & Integration Guide in VELARIX.md](VELARIX.md#developer-onboarding--integration-guide) for examples with LangChain and LlamaIndex.

### 1. Start the Velarix Kernel
Run the server locally using Docker or Go.
```bash
# Set a mandatory encryption key for production-grade security
export VELARIX_ENCRYPTION_KEY="your-32-byte-secure-key-here"
export VELARIX_ENV="dev" # Use dev mode to bypass strict checks for local testing
go run main.go
```
The kernel is now ready at `http://localhost:8080/v1`. All clients assume this versioned base path.

### 2. Connect with the Python SDK
Install the SDK and initialize a clinical reasoning session (defaults to the versioned base URL).
```python
from velarix import VelarixClient

client = VelarixClient(base_url="http://localhost:8080/v1", api_key="your_vx_key")
session = client.session("patient_case_001")

# Assert a medical premise (Root Fact)
session.observe("hipaa_consent_signed", payload={"form_version": "2026.1"})

# Derive a belief (Child Fact)
session.derive(
    "phi_processing_allowed",
    justifications=[["hipaa_consent_signed"]],
    payload={"scope": "full_records"}
)

# Retract the premise: Child facts collapse automatically
session.invalidate("hipaa_consent_signed")
```

### 3. Launch the Control Plane
Visualize the causal graph and audit reasoning in real-time.
```bash
cd console
npm install
npm run dev
```
Open `http://localhost:5173` to view the **3D Exploded-View Architecture** and retrace the lineage of every belief.

## 🧩 Integration Patterns

Velarix supports multiple levels of integration for AI agents, from simple drop-in replacements to complex reasoning co-processors:
- **Level 1: OpenAI Adapter** (Zero-code migration)
- **Level 2: Epistemic RAG** (Context injection)
- **Level 3: Manual Fact Management** (Causal chains)
- **Level 4: Real-time Truth Monitoring** (SSE)
- **Level 5: Compliance & Explainability** (Audit trails)
- **Level 6: Embedded Co-processor** (Sidecar mode)

For full code examples and implementation details, see [INTEGRATION_PATTERNS.md](./INTEGRATION_PATTERNS.md).

## Hardened Architecture

Velarix is built for production reliability:
- **Tenant Isolation**: Strict `OrgID` enforcement on every API handler prevents cross-tenant data leaks.
- **Hybrid Boot**: Combines binary snapshots with journal replays for sub-second startup of massive sessions.
- **Durable Journaling**: Every write is synchronized to disk (`Sync()`) to survive system crashes.
- **Actor Attribution & Admin Audit**: Every belief revision tracks the specific API key or User; admin actions (keys, backups, restores, config) are journaled with actor_id and timestamp.
- **Rate Limits with Persistence**: Per-key quotas are stored in BadgerDB and enforced across restarts.
- **Slice Controls**: `GET /v1/s/{id}/slice` honors `max_facts` to bound prompt size.

## Project Structure

- `/api`: Versioned REST API (`/v1`) with structured JSON logging.
- `/core`: The Epistemic Engine (Dominator Trees & Causal Logic).
- `/store`: Hardened BadgerDB v4 storage with AES-256 encryption.
- `/sdks`: Native Python and TypeScript clients with resource-efficient connection pooling.
- `/console`: React-based Control Plane with world-class motion design and graph visualization.
- `/docs`: OpenAPI/Swagger artifacts; runtime serves docs from `/docs/openapi.yaml`.

---
*Velarix: Building the trust layer for autonomous healthcare.*

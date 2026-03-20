# Velarix: The Epistemic State Layer for Healthcare AI

Velarix is a production-hardened belief-tracking engine designed for AI agents operating in regulated, high-stakes environments. It replaces logically flat memory (vector DBs) with a **Stateful Logical Graph** that enforces reasoning integrity.

In healthcare, agents must not only "remember" context but also **invalidate** state when policy, consent, or lab data changes. Velarix ensures that every agent decision is auditable, deterministic, and cryptographically attributed to a verified actor.

## Why Velarix?

Traditional vector databases fail in healthcare because they lack a causal model:
- **Causal Collapse**: If a patient withdraws HIPAA consent, every downstream belief dependent on that consent must be purged instantly. Velarix handles this with $O(1)$ pruning.
- **Audit Provenance**: Every state change is logged with an `actor_id` and timestamp, exportable as SHA-256 verified SOC2/HIPAA compliance reports.
- **Deterministic Reliability**: Logic-safe propagation ensures agents never act on stale or retracted medical premises.

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

# VELARIX: Deep Epistemic Architecture

Velarix is a stateful logical engine that provides a **deterministic conscience** for AI agents. It bridges the gap between probabilistic LLM outputs and the strict reasoning requirements of regulated industries like healthcare.

## 🏗️ The 7 Layers of Velarix (Hardened)

1.  **Epistemic Kernel (Go)**:
    -   Deterministic belief propagation ($AND=min, OR=max$).
    -   Real-time cycle detection using DFS path tracing.
    -   $O(1)$ Causal Invalidation via Dominator Tree theory, ensuring state consistency at scale.

2.  **Durable Persistence (BadgerDB v4)**:
    -   **AES-256 Encryption**: Mandatory in production, secured at rest.
    -   **Hybrid Boot**: Optimized startup that loads the latest binary snapshot and replays only the trailing journal entries.
    -   **Synchronous IO**: Every write is synchronized (`Sync()`) to ensure data survives system or power failures.

3.  **The Interceptor Layer**:
    -   Drop-in OpenAI & AsyncOpenAI adapters for seamless medical document processing.
    -   Schema-enforced fact extraction via tool-forcing.
    -   Automatic capture of model provenance and confidence metadata.

4.  **Security Infrastructure**:
    -   **Tenant Isolation**: Strict `OrgID` enforcement on every endpoint to prevent data leaks.
    -   **Actor Tracking**: Every state-modifying action is attributed to a specific API Key or User ID.
    -   **Admin Audit Trail**: Key lifecycle, config changes, backups, and restores are journaled with `actor_id`, timestamp, and payload.
    -   **Persistent Rate Limiting**: Request quotas are stored in BadgerDB to prevent brute-force attacks across restarts.

5.  **Observability Suite**:
    -   **Living Neural Graph**: Visualizes dependency flows with real-time causal particle effects.
    -   **Time-Travel Replay**: Journal-driven simulation mode for debugging complex invalidation cascades.
    -   **Blast Radius Analysis**: Quantitative "What If" impact reports for clinical premised retractions.

6.  **Integration Ecosystem**:
    -   Native LangGraph Checkpointers for persistent healthcare agent memory.
    -   LlamaIndex Epistemic Retrievers for belief-filtered RAG workflows.
    -   Full Async/Sync parity across Python and TypeScript SDKs.

7.  **Operations & Compliance**:
    -   **Versioned API**: Strictly enforced `/v1` endpoints for contract stability.
    -   **Structured Logging**: `slog`-based JSON output for enterprise log aggregation (Datadog/ELK).
    -   **Audit Exports**: One-click SOC2 compliance reports (PDF/CSV) with SHA-256 integrity verification.

---

## 🛡️ Production Principles

*   **Auditability by Default**: If a belief changed, we know exactly who changed it and why.
*   **Safety Over Speed**: We sacrifice sub-microsecond latency for disk-sync durability and encryption integrity.
*   **Zero-Drift Contracts**: The API versioning ensures that SDKs deployed in the field never experience breaking changes silently.

---

## 🚀 Developer Onboarding & Integration Guide

Velarix is designed to integrate cleanly with your existing agents without dictating their internal reasoning loops. Use this quickstart to drop Velarix into a production LangChain or LlamaIndex application.

### 1. Installation

Install the Python SDK into your environment:
```bash
pip install velarix-sdk
```

### 2. Configuration & Initialization

Ensure your server is running or configure the client to spin up the sidecar locally:

```python
import os
from velarix.client import VelarixClient

# In a cloud environment, pass the URL and API Key
client = VelarixClient(
    base_url=os.getenv("VELARIX_URL", "http://localhost:8080/v1"), 
    api_key=os.getenv("VELARIX_API_KEY")
)
```

### 3. Creating a Session Boundary

Wrap a discrete unit of clinical reasoning in a `session`:

```python
with client as c:
    # We scope reasoning to a specific encounter or patient request
    session = c.session("encounter_xyz")
    
    # 1. Provide the Root Premises (Inputs/Consents/Labs)
    session.observe("patient_consent", payload={"type": "hipaa_signed"})
    session.observe("lab_result_01", payload={"value": "elevated_WBC"})

    # 2. Derive logic using your agent
    # In a real app, an LLM call generates this insight. Velarix just records the dependency.
    session.derive("administer_antibiotics", justifications=[["patient_consent", "lab_result_01"]])

    # 3. Check State
    # The 'slice' endpoint returns ONLY the valid facts for prompt injection context and supports `max_facts` to bound prompt size.
    valid_context = session.get_slice(format="json")
```

### 4. Handling Clinical Retractions

If a patient revokes consent or a lab result is corrected, simply invalidate the root fact. Velarix will deterministically collapse all dependent decisions instantly.

```python
# The patient revokes consent:
session.invalidate("patient_consent")

# The 'administer_antibiotics' fact automatically transitions to an invalid state.
# Your next `session.get_slice()` call will omit it, preventing the agent from acting on retracted data.
```

---
*Velarix: Making AI reasoning auditable, reliable, and logical.*
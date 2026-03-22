# Velarix Integration Patterns

This document outlines the various ways to integrate Velarix (CasualDB) into AI agent workflows, ranging from simple drop-in replacements to complex, real-time reasoning co-processors.

---

## 📈 Level 1: The One-Line Swap (OpenAI Adapter)
**Best for:** Existing OpenAI-based agents that need memory without code changes.

Velarix provides an interceptor that automatically injects valid facts into the system prompt and extracts new observations using tool calling.

### Python
```python
# Before: from openai import OpenAI
from velarix.adapters.openai import OpenAI

# Initialize with a Velarix Session ID
client = OpenAI(velarix_session_id="research_task_001")

# Velarix automatically:
# 1. Injects 'Current Beliefs' into the system message.
# 2. Provides the 'record_observation' tool to the model.
# 3. Persists model outputs as justified facts in the DB.
response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Analyze the patient's heart rate."}]
)
```

---

## 🔍 Level 2: Epistemic RAG (Context Injection)
**Best for:** Standard RAG pipelines where you want to ensure the agent only sees "verified" or "current" data.

### Python
```python
from velarix import VelarixClient

client = VelarixClient()
session = client.session("session_123")

# Get the 'Truth Slice' in Markdown format for LLM context
context = session.get_slice(format="markdown", max_facts=20)

prompt = f"Use these verified facts to answer: {context}\nUser: What is the status?"
```

### TypeScript
```typescript
import { VelarixClient } from '@velarix/sdk';

const client = new VelarixClient();
const session = client.session('session_123');

const context = await session.getSlice('markdown');
// Inject into your prompt template...
```

---

## 🧠 Level 3: Manual Fact Management (Full Control)
**Best for:** Complex agents that need to build explicit reasoning chains and handle retractions.

### Python
```python
# 1. Assert a Root Premise (Observation)
session.observe("sensor_01_high", payload={"value": 98, "threshold": 90})

# 2. Derive a Conclusion (Justified Fact)
# This fact ONLY stays valid if 'sensor_01_high' stays valid.
session.derive(
    fact_id="alert_needed",
    justifications=[["sensor_01_high"]],
    payload={"severity": "CRITICAL"}
)

# 3. Retract (Invalidate)
# Automatically invalidates 'alert_needed' via Causal Propagation.
session.invalidate("sensor_01_high")
```

### TypeScript
```typescript
await session.observe('premise_1', { data: '...' });
await session.derive('conclusion_1', [['premise_1']], { action: 'notify' });
await session.invalidate('premise_1');
```

---

## ⚡ Level 4: Real-time Truth Monitoring (SSE)
**Best for:** Multi-agent systems or UIs that need to react instantly when a fact is retracted.

### TypeScript
```typescript
const stopListening = session.listen((event) => {
    console.log(`Fact ${event.fact_id} changed to status ${event.status}`);
    if (event.status === 0) {
        cancelAgentTask(event.fact_id); // Stop agent if its premise failed
    }
});
```

---

## 🛡️ Level 5: Compliance & Explainability
**Best for:** Regulated industries (Healthcare, Finance) requiring audit trails.

### Python
```python
# Why did the agent decide this?
explanation = session.get_fact("alert_needed")
print(f"Causal Path: {explanation['justification_sets']}")

# Generate SOC2-compliant Audit Export
# Available via API: /v1/s/{id}/export?format=pdf
```

---

## 🚀 Level 6: Embedded Co-processor (Sidecar Mode)
**Best for:** Local-first apps, edge devices, or high-performance Python environments.

Velarix can run as a managed subprocess (Sidecar) so you don't need to host a separate server.

### Python
```python
from velarix import VelarixClient

# Starts the Go binary automatically on a random local port
with VelarixClient(embed_mode=True) as client:
    session = client.session("local_reasoning")
    session.observe("local_fact")
    # ... session logic ...
# Binary is gracefully killed on exit
```

---

## Integration Summary Table

| Feature | Level | Use Case | Implementation |
| :--- | :--- | :--- | :--- |
| **OpenAI Adapter** | 1 | Zero-code Migration | `velarix.adapters.openai` |
| **`get_slice`** | 2 | Verified RAG | `session.get_slice()` |
| **`derive`** | 3 | Causal Chains | `session.derive(id, [[p1, p2]])` |
| **`listen`** | 4 | Reactive Agents | `session.listen(callback)` |
| **`get_why`** | 5 | Explainable AI | `session.get_fact(id)` |
| **`embed_mode`** | 6 | Local Performance | `VelarixClient(embed_mode=True)` |

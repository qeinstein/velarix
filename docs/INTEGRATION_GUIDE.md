# Velarix Integration Guide

This guide describes how to integrate Velarix into your existing AI agent workflows. Whether you want a drop-in replacement for OpenAI or a custom integration via our SDKs, Velarix is designed to be the "source of truth" for your agents.

## 🚀 The One-Line Swap (Recommended)

The fastest way to get started is by using the Velarix OpenAI Adapter. This provides a drop-in replacement for the standard `openai` library.

### OpenAI Adapter
```python
# Before: from openai import OpenAI
from velarix.adapters.openai import OpenAI

# Initialize with a Velarix Session ID
client = OpenAI(velarix_session_id="session_123")

# Standard OpenAI call - Velarix automatically handles:
# 1. Injecting valid facts into the system prompt
# 2. Extracting new observations into long-term memory
# 3. Tracking justifications (causal links)
response = client.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Patient reports severe chest pain."}]
)
```

## 🐍 Python SDK (Manual Control)

Initialize the client and session:
```python
from velarix.client import VelarixClient

client = VelarixClient(base_url="http://localhost:8080/v1", api_key="your_vx_key")

with client as c:
    session = c.session("patient_case_001")
    
    # Assert a root fact
    session.observe("patient_consent", payload={"type": "hipaa_signed"})
    
    # Derive a fact based on the root fact
    session.derive(
        "phi_processing",
        justifications=[["patient_consent"]],
        payload={"scope": "full_records"}
    )
```

## 🏗 Framework Integrations

Velarix provides native support for several common LLM frameworks.

### LangGraph

Velarix can be used as a **persistent checkpointer** or **state layer** for LangGraph agents.

```python
from velarix.integrations.langgraph import VelarixSaver

# Initialize the Velarix-backed checkpointer
saver = VelarixSaver(client=client)

# Use it when compiling your graph
app = workflow.compile(checkpointer=saver)
```

### LlamaIndex

Use the **Velarix Epistemic Retriever** to filter your RAG results based on current belief state.

```python
from velarix.integrations.llamaindex import EpistemicRetriever

retriever = EpistemicRetriever(
    session=session,
    vector_retriever=my_vector_retriever
)

# Only returns results that are consistent with Velarix's valid fact state
nodes = retriever.retrieve("What is the patient's current treatment plan?")
```

## 🤖 Context Injection

The most common integration pattern is using the `slice` endpoint to provide the "current truth" to your agent's system prompt.

### Prompt Construction

When your agent starts a turn, query the `slice` of valid facts:
```python
valid_context = session.get_slice(format="markdown", max_facts=10)

system_prompt = f"""
You are a healthcare assistant. Only use the following verified facts for your reasoning:
{valid_context}
"""
```

This pattern ensures the LLM never hallucinates or relies on retracted data.

---
*Velarix: Elevating AI agents from probabilistic to deterministic.*

# Velarix Integration Guide

This guide describes how to integrate Velarix into your existing AI agent workflows using our native SDKs and common LLM frameworks.

## 🐍 Python SDK

Install the Velarix Python SDK:
```bash
pip install velarix-sdk
```

### Basic Usage

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

# Velarix Integration Guide

This guide describes how to integrate Velarix into production AI agent workflows. Velarix is designed around a shared runtime core plus native framework/provider adapters so memory, reasoning controls, and audit behavior stay consistent across integrations.

> **Note:** For a structured overview of all integration levels (Level 1 to 6), see [INTEGRATION_PATTERNS.md](../INTEGRATION_PATTERNS.md).

## 🚀 Production Pattern (Recommended)

For long-term production use, Velarix should sit behind native framework/provider adapters that all reuse the same runtime contract:

1. Fetch current Velarix context
2. Inject the epistemic protocol and tools
3. Execute the model/framework call
4. Persist observations and explanations through the Velarix session API

The OpenAI adapter remains available for compatibility, but it now delegates to the shared runtime rather than owning its own side logic.

### OpenAI Adapter
```python
# Before: from openai import OpenAI
from velarix.adapters.openai import OpenAI

# Initialize with a Velarix Session ID
client = OpenAI(velarix_session_id="session_123")

# Standard OpenAI call - Velarix automatically handles:
# 1. Injecting valid facts into the system prompt
# 2. Registering Velarix reasoning tools
# 3. Persisting observations through the Velarix session API
response = client.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Patient reports severe chest pain."}]
)
```

### Shared Runtime

If you are implementing your own provider or framework adapter, build it on the shared chat runtime instead of duplicating prompt/tool logic inside each SDK surface.

```python
from velarix import VelarixChatRuntime

session = client.session("session_123")
runtime = VelarixChatRuntime(session, source="my_provider")

prepared = runtime.prepare_params({"messages": [{"role": "user", "content": "Hello"}]})
response = provider.chat.completions.create(**prepared)
runtime.process_response(response)
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

Velarix provides native support for framework extension points where possible.

### LangChain

Use the native LangChain adapter when you want Velarix behavior on a framework model surface instead of an OpenAI-specific shim.

```python
from langchain_openai import ChatOpenAI
from velarix.client import VelarixClient
from velarix.integrations.langchain import VelarixLangChainChatModel

client = VelarixClient(base_url="http://localhost:8080", api_key="your_vx_key")
session = client.session("agent_session")

base_model = ChatOpenAI(model="gpt-4o-mini")
model = VelarixLangChainChatModel(model=base_model, session=session)

response = model.invoke("What are the current validated facts?")
```

### LangChainJS

The TypeScript SDK exposes the same pattern for LangChainJS.

```ts
import { HumanMessage } from "@langchain/core/messages";
import { ChatOpenAI } from "@langchain/openai";
import { VelarixClient, VelarixLangChainChatModel } from "velarix-sdk";

const client = new VelarixClient({ baseUrl: "http://localhost:8080", apiKey: "your_vx_key" });
const session = client.session("agent_session");

const baseModel = new ChatOpenAI({ model: "gpt-4o-mini", apiKey: process.env.OPENAI_API_KEY });
const model = new VelarixLangChainChatModel(baseModel, session);

const response = await model.invoke([new HumanMessage("What are the current validated facts?")]);
```

### LangGraph

Velarix can be used as a **persistent checkpointer** or **state layer** for LangGraph agents.

```python
from velarix.integrations.langgraph import VelarixLangGraphMemory

# Initialize the Velarix-backed checkpointer
memory = VelarixLangGraphMemory(client=client)

# Use it when compiling your graph
app = workflow.compile(checkpointer=memory)
```

### LlamaIndex

Use the **Velarix Epistemic Retriever** to filter your RAG results based on current belief state.

```python
from velarix.integrations.llamaindex import VelarixRetriever

retriever = VelarixRetriever(
    session_id="patient_case_001",
    client=client
)

# Returns the logically valid fact slice as retrievable nodes
nodes = retriever.retrieve("What is the patient's current treatment plan?")
```

## 🤖 Context Injection

The most common integration pattern is using the `slice` endpoint to provide the "current truth" to your agent's system prompt.

### Prompt Construction

When your agent starts a turn, query the `slice` of valid facts:
```python
valid_context = session.get_slice(format="markdown", max_facts=10)

system_prompt = f"""
You are an internal approvals assistant. Only use the following verified facts for your reasoning:
{valid_context}
"""
```

This pattern ensures the LLM never hallucinates or relies on retracted data.

---
*Velarix: Elevating AI agents from probabilistic to deterministic.*

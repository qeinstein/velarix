---
title: "Python SDK: Integrations And MCP"
description: "Use the shipped integrations for LangChain, LangGraph, CrewAI, LlamaIndex, and the MCP server that exposes Velarix context and reasoning tools."
section: "Python SDK"
sectionOrder: 4
order: 3
---

# Integrations and MCP

The SDK includes several adapters in `sdks/python/velarix/integrations/` plus an MCP server in `mcp_server.py`. Each one maps existing framework concepts onto Velarix sessions instead of re-implementing the reasoning engine.

## LangChain

`VelarixLangChainChatModel` and `wrap_langchain_model(...)` decorate a LangChain chat model with Velarix-backed context handling.

Example:

```python
from langchain_openai import ChatOpenAI
from velarix.client import VelarixClient
from velarix.integrations.langchain import wrap_langchain_model

velarix = VelarixClient(base_url="http://localhost:8080", api_key="vxk_test")
base_model = ChatOpenAI(model="gpt-4.1-mini")
model = wrap_langchain_model(base_model, velarix, session_id="langchain-demo")

result = model.invoke("Can I release payment for invoice inv-1042?")
print(result.content)
```

## LangGraph

`VelarixLangGraphMemory` stores LangGraph checkpoint state in Velarix session history and exposes an `epistemic_check_node`.

Important behavior from the integration source:

- writes `langgraph_checkpoint` history entries
- writes `langgraph_pending_writes` history entries
- `epistemic_check_node` inspects `current_plan_fact_id`
- if that fact resolves below the confidence threshold, it sets `needs_replanning`

Example:

```python
from velarix.client import VelarixClient
from velarix.integrations.langgraph import VelarixLangGraphMemory, epistemic_check_node

client = VelarixClient(base_url="http://localhost:8080", api_key="vxk_test")
memory = VelarixLangGraphMemory(client.session("langgraph-demo"))

state = {
    "current_plan_fact_id": "plan.current",
    "needs_replanning": False,
}
next_state = epistemic_check_node(state, memory.session)
print(next_state)
```

As with the runtime helpers, the history-writing behavior assumes a deployment that exposes the optional history write route.

## CrewAI

`VelarixCrewAIMemory` provides three core helpers:

- `build_context`
- `augment_description`
- `record_observation`

Example:

```python
from velarix.client import VelarixClient
from velarix.integrations.crewai import VelarixCrewAIMemory

client = VelarixClient(base_url="http://localhost:8080", api_key="vxk_test")
memory = VelarixCrewAIMemory(client.session("crewai-demo"))

context = memory.build_context(query="invoice approval")
description = memory.augment_description("Review the current payment request.")
print(context)
print(description)
```

## LlamaIndex

`VelarixRetriever` adapts session slices and semantic search into a LlamaIndex retriever.

Example:

```python
from velarix.client import VelarixClient
from velarix.integrations.llamaindex import VelarixRetriever

client = VelarixClient(base_url="http://localhost:8080", api_key="vxk_test")
retriever = VelarixRetriever(client.session("llamaindex-demo"))

nodes = retriever.retrieve("payment release evidence")
print(nodes)
```

## MCP server

The MCP server lives in `sdks/python/velarix/integrations/mcp_server.py` and uses `FastMCP`.

Tools exposed:

- `assert_fact`
- `get_fact`
- `explain_reasoning`

Resources exposed:

- `velarix://session/{session_id}/context`

Typical use:

```bash
python -m velarix.integrations.mcp_server
```

Then point your MCP-capable client at that server and bind it to a Velarix deployment with the usual `VELARIX_BASE_URL` and `VELARIX_API_KEY` environment variables.

## Integration choice guidance

- Use LangChain integration when you already use LangChain model abstractions.
- Use LangGraph integration when you need checkpointed graph execution and replanning hooks.
- Use CrewAI integration when you want a memory layer for agents.
- Use LlamaIndex integration when you want Velarix-backed retrieval.
- Use the MCP server when you need tool-based access from an external MCP client.

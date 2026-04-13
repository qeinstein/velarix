# Python SDK

The Python SDK is the main client surface for Velarix.

It provides:

- synchronous and asynchronous clients
- session-scoped fact, slice, explanation, consistency, reasoning-chain, extract-and-assert, and decision helpers
- runtime helpers for model integration
- `VelarixGateway` for tool-call audit capture
- optional LangChain, LangGraph, CrewAI, and LlamaIndex surfaces

## Install

Base client:

```bash
pip install -e ./sdks/python
```

Optional extras:

```bash
pip install -e './sdks/python[langgraph]'
pip install -e './sdks/python[crewai]'
pip install -e './sdks/python[llamaindex]'
pip install -e './sdks/python[langchain]'
```

## Quickstart

```python
from velarix import VelarixClient

client = VelarixClient(base_url="http://localhost:8080", api_key="dev-admin-key")
session = client.session("hello_world")

session.observe("fact_1", {"summary": "Vendor 17 passed KYB"})
session.derive(
    "fact_2",
    [["fact_1"]],
    {"summary": "Payment recommendation is supported"},
)

facts = session.get_slice(
    query="payment recommendation vendor verification",
    strategy="hybrid",
    include_dependencies=True,
)
```

## Execution Pattern

The production pattern is:

1. observe or derive the facts
2. create the decision
3. call `execute_check`
4. execute only while the decision remains valid

## Integration Modules

- `velarix.runtime`
- `velarix.gateway`
- `velarix.adapters.openai`
- `velarix.integrations.langchain`
- `velarix.integrations.langgraph`
- `velarix.integrations.crewai`
- `velarix.integrations.llamaindex`

## Product Guidance

The SDK is optimized for session-scoped operational workflows, not generic long-term memory dumping.

Use slices when an agent needs current belief state.
Use decisions when an agent is approaching a real side effect.

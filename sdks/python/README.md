# Python SDK

The Python SDK is the main client surface for Velarix.

It provides:

- synchronous and asynchronous clients
- session-scoped fact, slice, explanation, and decision helpers
- runtime helpers for model integration
- optional LangGraph, CrewAI, LlamaIndex, and LangChain surfaces

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

## Global Facts

Global facts are org-wide assertions shared across sessions (admin-only endpoints).

```python
from velarix import VelarixClient

client = VelarixClient(base_url="http://localhost:8080", api_key="dev-admin-key")

client.global_facts.assert_fact("today", {"date": "2026-04-13"})
items = client.global_facts.list()
client.global_facts.retract("today")
```

## Execution Pattern

The production pattern is:

1. observe or derive the facts
2. create the decision
3. call `execute_check`
4. execute only while the decision remains valid

## Integration Modules

- `velarix.runtime`
- `velarix.adapters.openai`
- `velarix.integrations.langgraph`
- `velarix.integrations.crewai`
- `velarix.integrations.llamaindex`

## Product Guidance

The SDK is optimized for session-scoped operational workflows, not generic long-term memory dumping.

Use slices when an agent needs current belief state.
Use decisions when an agent is approaching a real side effect.

# Velarix Python SDK

The official Python client for Velarix: the epistemic state layer for regulated AI agents.

## Installation

```bash
pip install velarix
```

## The One-Line Swap (OpenAI Adapter)

The easiest way to integrate Velarix into an existing OpenAI-based project is through our adapter.

```python
# Before: from openai import OpenAI
from velarix.adapters.openai import OpenAI

# Specify a session ID during initialization or per-call
client = OpenAI(velarix_session_id="session_456")

# That's it. Velarix now intercepts chat completions to inject context and extract facts.
response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Patient is presenting with acute shortness of breath."}]
)
```

## Manual Usage (Direct Client)

## Features

- **Parity**: Full support for both `sync` and `async` workflows.
- **Resource Efficiency**: Connection pooling via shared `httpx.AsyncClient`.
- **Interceptors**: Drop-in adapters for OpenAI and LangGraph.
- **Compliance**: Automatic injection of model provenance and confidence scores.

## Async Usage

```python
from velarix import AsyncVelarixClient

async with AsyncVelarixClient(api_key="...") as client:
    session = client.session("session_123")
    await session.observe("fact_1")
```

---
*Velarix: Building the trust layer for autonomous healthcare.*

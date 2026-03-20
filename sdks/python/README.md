# Velarix Python SDK

The official Python client for Velarix: the epistemic state layer for regulated AI agents.

## Installation

```bash
pip install velarix
```

## Quick Start

```python
from velarix import VelarixClient

# Initialize client with production-grade security
client = VelarixClient(
    api_key="vx_healthcare_prod_...",
    base_url="https://api.velarix.dev"
)

# Start a clinical reasoning session
session = client.session("patient_case_492")

# 1. Assert a Root Fact (e.g., Clinical Consent)
session.observe("hipaa_consent", payload={"status": "signed", "form": "v2.1"})

# 2. Derive Beliefs (e.g., Access to Records)
# If 'hipaa_consent' is ever invalidated, this belief collapses instantly.
session.derive(
    "phi_access_granted", 
    justifications=[["hipaa_consent"]], 
    payload={"level": "full_records"}
)

# 3. Retract & Collapse
session.invalidate("hipaa_consent")
```

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

# Velarix: The Epistemic Layer for AI Agents

![Velarix](https://img.shields.io/badge/Status-Alpha-orange) ![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)

The current AI agent landscape is plagued by "Agent Drift" and "Context Hallucination." Frameworks like LangChain and CrewAI focus on *orchestration* (how to move), but they lack *epistemology* (what to believe).

**Velarix** is a standalone "Belief Server" and Truth Maintenance System (TMS) for your AI agents. It ensures that your agents never act on stale, hallucinated, or retracted information. 

## Quickstart (The "Import Swap")

Velarix requires zero prompt-engineering or manual schema definitions. Just swap your OpenAI import:

```python
from velarix.adapters.openai import OpenAI

# Your agent now has a memory!
client = OpenAI(velarix_session_id="research-session-1")

response = client.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Analyze the market data..."}]
)
```

## Why Velarix?

* **Infinite Logic Pruning**: Instead of doing expensive RAG checks on every step, Velarix uses a **Dominator Tree** algorithm. If a foundational "Root Fact" is retracted, Velarix invalidates every downstream conclusion in $O(1)$ time.
* **Causal Traceability**: Every decision the agent makes is backed by an `explain_reasoning` trace. You can ask: *"What would the agent have done if Fact X was never discovered?"*
* **Epistemic Integrity**: LLMs are overconfident. Velarix automatically caps the confidence of root assertions and requires multi-source verification to reach "certainty."

## Running the Belief Server

Velarix runs a high-performance Go backend.

```bash
docker-compose up -d
```

Or run it natively in Lite Mode:
```bash
go run main.go --lite
```

## Documentation
- [How it Works (The Theory)](docs/theory.md)
- [Python SDK Docs](sdks/python/README.md)

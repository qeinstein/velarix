# Velarix Healthcare Demo

This directory contains proofs-of-concept for Velarix in high-stakes reasoning environments.

Available integration demos:
1. `agent_pivot.py`: core causal invalidation flow
2. `langchain_integration.py`: native LangChain model wrapper
3. `langgraph_integration.py`: LangGraph checkpoint integration
4. `llamaindex_integration.py`: LlamaIndex retriever integration
5. TypeScript SDK tests also cover a native LangChainJS wrapper and OpenAI adapter under `sdks/typescript/tests/`

## The Problem
In healthcare, AI agents often act on temporary or conditional data (e.g., preliminary lab results or active patient consent). If that data is retracted or updated, standard "flat" agent memory fails to purge the downstream reasoning, leading to clinical safety violations or HIPAA breaches.

## The Demo: Clinical Consent Retraction
This script (`agent_pivot.py`) simulates a medical data-processing agent.
1. The agent observes a patient has signed a **HIPAA Consent Form**.
2. It derives multiple reasoning paths: **PHI_ACCESS_GRANTED**, **LAB_PROCESSING_ENABLED**, and **INSURANCE_VERIFICATION_ACTIVE**.
3. The patient suddenly **withdraws consent**.
4. We invalidate the root consent fact.
5. **The Result:** Velarix triggers an instant $O(1)$ causal collapse. Every reasoning chain dependent on that consent is invalidated before the agent can perform its next action.

## How to run

1. Start the Velarix Go Server from the project root:
```bash
# Ensure encryption is set for compliance mode
export VELARIX_ENCRYPTION_KEY="your-32-byte-secure-key-here"
export VELARIX_ENV="dev"
go run main.go
```

2. Open a new terminal and run the demo:
```bash
pip install -e ./sdks/python
python demo/agent_pivot.py
```

---
*Velarix: Building the trust layer for autonomous healthcare.*

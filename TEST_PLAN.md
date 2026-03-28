# Velarix System Validation Plan

This document outlines the end-to-end testing procedure to verify the Velarix Epistemic State Layer. These tests are designed to be performed by a user interacting with the system via the REST API and the provided SDKs (Python and TypeScript).

## Prerequisites

Before starting, ensure the following environment is configured:

1. Server is running: `go run main.go`
2. Environment Variables:
   - `VELARIX_ENCRYPTION_KEY`: A 32-byte secure key.
   - `VELARIX_JWT_SECRET`: A secure string for token signing.
   - `VELARIX_ENV`: Set to `dev` for initial local testing.
3. Access: API Key or JWT for authentication.

---

## Phase 1: Epistemic Lifecycle (Basic Usage)

Goal: Verify the core ability to manage beliefs and their relationships.

### Test 1.1: Session Initialization and Root Assertion
- Action: Create a new session and assert a Root Fact (e.g., "Patient has insurance").
- Validation: The fact should return a status of 1.0 (Valid) and appear in the session fact list.

### Test 1.2: Fact Derivation
- Action: Assert a Derived Fact that depends on the Root Fact (e.g., "Treatment is covered" depends on "Patient has insurance").
- Validation: The derived fact should automatically resolve to Valid because its dependency is satisfied.

### Test 1.3: Retrieval Formats
- Action: Fetch the Valid Slice using both `format=json` and `format=markdown`.
- Validation: JSON should contain structured metadata; Markdown should provide a human-readable summary suitable for LLM context injection.

---

## Phase 2: Causal Invalidation (Reasoning Engine)

Goal: Verify that the system correctly propagates "forgetting" through the graph.

### Test 2.1: The Chain Reaction
- Action: Create a dependency chain: A -> B -> C -> D.
- Action: Invalidate the Root Fact (A).
- Validation: Fact D must immediately return a status of 0.0 (Invalid) without manual intervention.

### Test 2.2: Pre-retraction Impact Analysis
- Action: Call the `/impact` endpoint for a Root Fact that supports multiple downstream facts.
- Validation: The system must return a report listing every fact ID that would be lost if that root were retracted.

### Test 2.3: The Frame Problem (Redundant Support)
- Action: Create a Fact C that is justified by [[Fact A], [Fact B]] (OR-of-ANDs logic).
- Action: Invalidate Fact A.
- Validation: Fact C must remain Valid because Fact B still supports it.
- Action: Invalidate Fact B.
- Validation: Fact C must now become Invalid.

---

## Phase 3: Persistence and Recovery (Durability)

Goal: Ensure data integrity across system failures.

### Test 3.1: Journal Replay
- Action: Assert several facts, then force-kill the server process (SIGKILL).
- Action: Restart the server and access the same session ID.
- Validation: All previously asserted facts must be present and logically consistent.

### Test 3.2: Hybrid Boot and Snapshots
- Action: Perform 55 mutations (assertions or invalidations) in a single session.
- Action: Restart the server.
- Validation: Check logs to verify the system loaded from a Binary Snapshot and then replayed the remaining journal entries.

---

## Phase 4: SDK and AI Framework Integrations

Goal: Confirm the Python and TypeScript SDKs correctly wrap the API logic.

### Test 4.1: OpenAI Interceptor (Python)
- Action: Use the `velarix.adapters.openai` wrapper to prompt an LLM.
- Action: Ask the LLM to remember a piece of information.
- Validation: Verify the LLM uses the `record_observation` tool and that the fact appears in the Velarix session automatically.

### Test 4.2: LangGraph Checkpointing (TypeScript/Python)
- Action: Use the `VelarixLangGraphMemory` saver within a LangGraph workflow.
- Action: Run a graph, stop it, and resume from a specific `thread_id`.
- Validation: The graph state must be successfully retrieved from Velarix facts.

### Test 4.3: LlamaIndex Epistemic Retrieval (Python)
- Action: Use `VelarixRetriever` to query a session.
- Action: Invalidate a fact in the session and run the retriever again.
- Validation: The invalidated fact must no longer appear in the retrieved nodes, ensuring the RAG context is always "True."

---

## Phase 5: Security and Multi-tenancy

Goal: Verify data isolation and protection.

### Test 5.1: Tenant Isolation
- Action: Create Session X with API Key A (Org 1).
- Action: Attempt to read Session X with API Key B (Org 2).
- Validation: The server must return a 403 Forbidden or 401 Unauthorized error.

### Test 5.2: Encryption at Rest
- Action: Shut down the server and open the `velarix.data` directory.
- Action: Search for strings contained in your asserted facts using a hex editor or `grep`.
- Validation: No plaintext fact data should be discoverable; it must be encrypted.

---

## Phase 6: Observability and Explanations

Goal: Verify the system can explain its own logic.

### Test 6.1: Causal Tracing (The "Why")
- Action: Call the `/explain` endpoint for a deep derived fact.
- Validation: The response must return a tree structure showing every supporting fact and its current status.

### Test 6.2: Real-time Event Streaming
- Action: Connect to the `/events` SSE endpoint for a session.
- Action: In a separate terminal, invalidate a root fact.
- Validation: The SSE stream should immediately push a `ChangeEvent` showing the status change of the root and all its children.

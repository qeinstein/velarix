# Step-by-Step Codebase Reading Guide for Velarix

To fully understand Velarix's architecture—a production-hardened Truth Maintenance System (TMS)—we recommend reading the code from the inside out. Start with the core logical data structures, move to the database/storage layer, up through the API server, and finally explore how external clients interact with the system via SDKs and the UI.

Here is the recommended step-by-step reading order:

## Phase 1: The Core Epistemic Engine (Go)
This is the heart of Velarix. It manages the in-memory causal graph of facts.
1. **`core/fact.go`**
   *Why:* Start here to understand the fundamental building blocks: the `Fact` and `JustificationSet` structs, and the `Status` enumerations.
2. **`core/engine.go`**
   *Why:* This contains the `Engine` struct and the core logic for asserting facts (`AssertFact`) and propagating status changes (`propagate`). This is the most crucial file in the backend.
3. **`core/dominators.go`**
   *Why:* Understand how Velarix achieves $O(1)$ impact analysis and invalidation cascades using the Lengauer-Tarjan/Dominator Tree algorithms.
4. **`core/cycles.go`**
   *Why:* See how the engine prevents agents from creating circular logic loops (DFS reachability).
5. **`core/explain.go`**
   *Why:* Review how the engine traverses backward to generate the "Why" causal chains for Explainable AI (XAI).

## Phase 2: Storage & Persistence (Go)
How Velarix ensures that belief states survive crashes and restarts.
6. **`store/journal.go`**
   *Why:* Defines the Write-Ahead Log (WAL) event types and structs used to append changes.
7. **`store/badger.go`**
   *Why:* Shows how the `Engine` state (Snapshots, Journals, API keys, and rate limits) is durably synced to BadgerDB.

## Phase 3: The API & Control Plane (Go)
How the external world communicates with the engine.
8. **`main.go`**
   *Why:* The main entrypoint. See how the server, storage, and configurations are initialized.
9. **`api/server.go`**
   *Why:* Read the HTTP routing logic, how sessions are mapped to `Engine` instances, hybrid booting (snapshots + journal replay), and how endpoints translate to Core actions.
10. **`api/auth.go`**
   *Why:* Understand tenant isolation, API key resolution, and JWT validation.
11. **`api/metrics.go`**
   *Why:* Quick glance at Prometheus telemetry integration.

## Phase 4: SDKs & Client Libraries (Python & TypeScript)
How AI Agents actually interact with Velarix.
12. **`sdks/python/velarix/client.py`**
    *Why:* Read the Python `VelarixClient` and `VelarixSession`. Pay special attention to the `SidecarManager` which allows running the Go binary natively within a Python process.
13. **`sdks/python/velarix/adapters/openai.py`**
    *Why:* The implementation of the "One-Line Swap". See how system prompts are injected and how `tool_calls` for `record_observation` are intercepted to automatically assert facts.
14. **`sdks/python/velarix/integrations/`** (`langgraph.py` & `llamaindex.py`)
    *Why:* Examples of hooking Velarix into established Agent frameworks as checkpointers or retrievers.
15. **`sdks/typescript/src/types.ts` & `sdks/typescript/src/client.ts`**
    *Why:* The TS equivalent of the client. Important to note how Server-Sent Events (SSE) are handled via the `listen` method for real-time reactivity.

## Phase 5: The Console UI (React/TypeScript)
How humans audit the graph.
16. **`console/src/pages/NeuralGraph.tsx`**
    *Why:* The core visualization component. See how facts and justification sets are rendered visually.
17. **`console/src/pages/Dashboard.tsx` & `App.tsx`**
    *Why:* General layout, routing, and session management in the UI.

## Phase 6: Practical Demos & Tests
Putting it all together.
18. **`demo/agent_pivot.py`**
    *Why:* An end-to-end script showcasing a "hallucination cascade" and instant pruning when a premise is invalidated.
19. **`tests/`** (e.g., `tests/core_test.go`, `tests/e2e_test.go`)
    *Why:* Read the tests to understand the edge cases and expected behaviors of complex justification structures.

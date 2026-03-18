# Velarix: The Epistemic Orchestration Layer

Velarix is a high-performance logic firewall for AI agents. It solves the **Stale Context Hallucination** problem by treating agent memory not as a flat log of facts, but as a dynamic dependency graph of justified beliefs.

## The Problem: "Context Soup"
Modern agents accumulate facts in append-only vector stores. When a foundational premise changes (e.g., a user pivots from React 17 to 18), the agent’s context window becomes a "soup" of old and new assumptions. The LLM then hallucinates by trying to reconcile these contradictions.

## The Solution: Deterministic Belief Revision
Velarix implements **Incremental Support Accounting** and **Dominator-based Pruning**. When a premise is invalidated, Velarix logically collapses the entire downstream chain of dependent thoughts in **O(1)** time on the write-path. This ensures your agent's context window is always mathematically guaranteed to be consistent.

---

## Architecture: The 7 Layers of Velarix

1.  **Epistemic Kernel (Go)**: A dual-layer engine that maintains belief support counters and a secondary Dominator Tree for instant pruning.
2.  **Multi-Tenant Partitioning**: Isolated "Reasoning Sessions" that prevent context leakage between users.
3.  **High-Performance Persistence (BadgerDB)**: $O(K)$ session lookups via prefixed binary keys, ensuring history retrieval scales even with millions of sessions.
4.  **Schema Enforcement Layer**: Configurable "Strict" or "Warn" modes that reject or flag malformed agent outputs before they poison the graph.
5.  **Context Slicing API**: Dynamically generates "Prompt-Ready" snapshots of the current valid truth in JSON or Markdown.
6.  **The LLM Interceptor (OpenAI Adapter)**: A drop-in replacement for the OpenAI client that automates context injection and fact extraction.
7.  **Neural Graph Visualizer**: A production-grade observability dashboard for debugging and auditing agent reasoning in real-time.

---

## Integration in < 10 Minutes

### 1. Get your API Key
Visit [velarix.dev/keys](https://velarix.dev/keys) to generate your unique access token. You point your SDK at our hosted orchestrator—no infrastructure management required.

### 2. Swap Your SDK Import
Velarix is a drop-in replacement. You don't need to refactor your reasoning loops.

```python
# from openai import OpenAI
from velarix.adapters.openai import OpenAI

# Initialize with your session ID
client = OpenAI(
    api_key="your-openai-key",
    velarix_api_key="vx_...", 
    velarix_session_id="user_456"
)
```

# That's it. 
# Injection and extraction are now automatic.
response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "The database is now on Port 5432."}]
)
```

---

## Technical Status & Roadmap

### Currently Deferred
- **Client-Side Caching**: Currently, every `get_slice` is an API hit. We are deferring local LRU caching of the context slice to prioritize data consistency.
- **AsyncIO Support**: The Python SDK is currently synchronous. Async support is prioritized for the v0.2 release.
- **Complex Multi-Turn Extraction**: The interceptor currently handles the first tool call. Multi-step reasoning loops are handled but require manual session binding for now.

### Known Limitations
- **Memory-First**: While BadgerDB handles persistence, the active logic graph for a session is held in memory for performance. Large graphs (>100k nodes per session) may require higher memory overhead.
- **Linear Replay**: Startup replay from BadgerDB is linear. Snapshotting is on the roadmap to accelerate cold boots.

### The Roadmap
- [ ] **v0.2**: Native LangChain & LlamaIndex integrations.
- [ ] **v0.3**: Semantic search over the "Valid Subset" of facts.
- [ ] **v0.4**: Cross-session "Global Knowledge" inheritance.
- [ ] **v1.0**: Distributed consensus for high-availability clusters.

**Velarix is taking compiler-grade math and weaponizing it for the age of Autonomous Agents. Stop paying for hallucinations.**

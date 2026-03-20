# Velarix Architecture

Velarix is a stateful logical engine that provides a **deterministic conscience** for AI agents. It bridges the gap between probabilistic LLM outputs and the strict reasoning requirements of regulated industries.

## 🏛️ System Overview

Velarix is built around a versioned API and a hardened Go kernel. It treats agent memory as a directed acyclic graph (DAG) of **Facts** and **Justifications**.

### The 7 Layers of Velarix

1.  **Epistemic Kernel (Go)**:
    -   Handles belief propagation and causal invalidation.
    -   Implements cycle detection and dominator tree analysis.
    -   $O(1)$ invalidation cascades for large-scale sessions.

2.  **Durable Persistence (BadgerDB v4)**:
    -   **AES-256 Encryption**: Mandatory in production for data at rest.
    -   **Hybrid Boot**: Optimized startup using binary snapshots and journal replays.
    -   **Synchronous IO**: Guarantees data survival across system crashes.

3.  **The Interceptor Layer**:
    -   Adapters for LLM frameworks (OpenAI, LangGraph, LlamaIndex).
    -   Automated fact extraction and provenance tracking.

4.  **Security Infrastructure**:
    -   **Tenant Isolation**: Strict `OrgID` partitioning on every endpoint.
    -   **Actor Tracking**: Attribution of every state change to a verified actor.
    -   **Admin Audit Trail**: Journaled history of administrative actions.

5.  **Observability Suite**:
    -   **Neural Graph Visualization**: Real-time rendering of belief dependencies.
    -   **Time-Travel Replay**: Debugging complex state transitions via journal replay.
    -   **Impact Analysis**: Quantifying the "blast radius" of retracted premises.

6.  **Integration Ecosystem**:
    -   Native SDKs (Python, TypeScript).
    -   Framework-specific retrievers and checkpointers.

7.  **Operations & Compliance**:
    -   **Versioned API**: `/v1` base for contract stability.
    -   **Structured JSON Logging**: Integration with enterprise log aggregators.
    -   **Compliance Exports**: SOC2/HIPAA-ready audit reports (PDF/CSV).

## 🧠 Epistemic Kernel

The core of Velarix is the **Epistemic Kernel**, which manages the lifecycle of **Facts**.

### Facts & Justifications

- **Fact**: A single piece of information or belief.
  - **Root Fact**: A manually asserted premise (e.g., "Patient signed HIPAA consent").
  - **Derived Fact**: A fact that depends on other facts (e.g., "PHI processing allowed").
- **Justification**: A set of parent facts that support a derived fact.
  - Velarix uses **OR-of-AND** logic: A fact is valid if at least one of its justification sets is fully valid.

### Causal Invalidation (Dominator Trees)

Velarix uses **Dominator Tree** theory to optimize state invalidation. If a root fact is invalidated, Velarix identifies all downstream facts that *exclusively* depend on that root and retracts them instantly.

This ensures that agents never act on stale or retracted data, maintaining **State Integrity** at all times.

## 💾 Persistence & Durability

Velarix uses **BadgerDB v4**, an LSM-tree based key-value store, for high-performance writes and reliable persistence.

### Hybrid Boot Strategy

To ensure sub-second startup times for large sessions, Velarix employs a hybrid boot strategy:
1.  **Snapshot**: Periodically, Velarix saves the entire state of a session as a binary snapshot.
2.  **Journal Replay**: On startup, the latest snapshot is loaded, and then any journal entries (assertions/invalidations) that occurred *after* the snapshot are replayed.

This combination provides both fast recovery and 100% data accuracy.

---
*Velarix: Deterministic logic for the age of autonomous agents.*

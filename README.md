# CausalDB

### A Reactive Justification Database for AI Agents

> **CausalDB is a database that stores *why* facts are true, not just *what* is true.**

Traditional databases persist state.
CausalDB persists **justified belief**.

---

## The Problem

Modern AI systems increasingly need **memory**:

* beliefs that persist across time
* conclusions derived from other conclusions
* decisions that must be revisable when assumptions change

Today, this memory is built on top of **traditional databases** that:

* store values without provenance
* overwrite conclusions instead of retracting them
* cannot explain *why* something is believed
* silently accumulate stale or invalid knowledge

This leads to:

* hallucinated long-term beliefs
* contradictions across sessions
* unsafe agent behavior
* explanations that are generated after the fact, not grounded in data

---

## The Core Idea

**A fact should not exist unless its reasons still hold.**

CausalDB enforces a single invariant:

> **A fact is valid if and only if at least one of its justification sets is fully valid.**

Everything else follows from this.

---

## What Makes CausalDB Different

CausalDB is not a SQL database, a graph database, or a vector database.

It introduces **new database semantics**:

| Traditional DB | CausalDB              |
| -------------- | --------------------- |
| Rows           | Claims                |
| Foreign keys   | Causal justifications |
| Delete         | Invalidate            |
| Logs           | First-class reasoning |
| State          | Justified belief      |

Instead of overwriting data, CausalDB **invalidates assumptions** and deterministically collapses dependent beliefs.

---

## Core Concepts

### Facts

A **Fact** is a claim the system may believe.

Facts can be:

* **Root facts** (axioms / assumptions)
* **Derived facts** (conclusions)

---

### Justification Sets (OR-of-ANDs)

Each fact may have one or more **justification sets**.

```text
Fact B is valid if:
  (A is valid)
  OR
  (C is valid AND D is valid)
```

This solves the *Frame Problem* and allows multiple independent explanations.

---

### Validity, Not Deletion

Facts are never deleted.

Instead:

* root assumptions are invalidated
* dependent facts collapse automatically
* historical reasoning is preserved

Truth is **maintained**, not mutated.

---

## Why This Matters for AI Agents

AI agents reason probabilistically, but **persist beliefs deterministically**.

Without causal structure, agents:

* cannot safely revise beliefs
* cannot tell which conclusions are fragile
* cannot explain decisions reliably

CausalDB gives agents:

* epistemic discipline
* safe long-term memory
* deterministic belief revision
* structural explanations

This does **not** make agents smarter —
it makes them **wrong less often**.

---

## Architecture Overview

CausalDB is a **single-node causal reasoning engine** with:

* deterministic ripple propagation
* cycle-safe justification graphs
* append-only persistence
* structural explanation queries

The engine is fully usable **without HTTP** and can be embedded directly into agent runtimes.

---

## API (Minimal)

```http
POST   /facts                    # assert a fact
POST   /facts/{id}/invalidate    # invalidate a root fact
GET    /facts/{id}               # fetch a fact
GET    /facts/{id}/why           # explain why it is valid
GET    /facts?valid=true         # list currently valid facts
```

There are:

* no updates
* no deletes
* no partial mutations

All change flows through **invalidation + re-derivation**.

---

## Example (Conceptual)

1. An agent asserts:

```json
"User is trustworthy"
```

Because:

* prior interactions
* assumption: `identity_verified == true`

2. Later, identity verification fails.

3. The root assumption is invalidated.

4. All dependent beliefs collapse automatically.

The agent does not “forget” —
it **understands why it no longer believes**.

---

## What This Is (and Is Not)

### This *is*:

* a new database invariant
* a reasoning-native data model
* a foundation for AI memory, audit, and safety

### This is *not*:

* a SQL replacement
* a high-throughput OLTP store
* a distributed system (yet)

CausalDB is designed to **sit beside** traditional databases, not replace them.

---

## Status

This repository contains:

* a complete causal database kernel
* deterministic propagation
* explanation engine
* append-only persistence
* agent-facing HTTP API

It is an MVP intended to demonstrate a **new database category**.

---

## One-Line Summary

> **CausalDB is a database where facts must justify their existence — and collapse when their reasons fail.**

---

## Next Steps

* Graph visualization of causal collapse
* Agent SDKs
* Persistence snapshots
* Distributed replication (research phase)

---

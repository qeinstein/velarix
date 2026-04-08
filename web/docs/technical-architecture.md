---
title: "Technical Architecture"
description: "High-level overview of how Velarix is built."
order: 3
---

# Technical Architecture

Velarix is built with a focus on simplicity, speed, and causality.

## Core Components

1. **Control Plane**: Manages sessions, users, and billing (if using the cloud version).
2. **Fact Engine**: Tracks facts, observations, and derivations. It maintains the causal graph.
3. **Adapters**: Lightweight wrappers around popular AI SDKs (like OpenAI, LangChain) that seamlessly inject session context and record reasoning chains.

## Data Flow

When an agent makes a decision:
1. The **Adapter** intercepts the context.
2. The **Fact Engine** verifies all underlying facts.
3. If any premise has changed, the reasoning chain is invalidated, prompting the agent to re-evaluate.
4. The updated state is persisted in either local storage (Badger) or a distributed store (Postgres/Redis).

## Infrastructure

- **Backend**: Go for high performance and concurrency.
- **Storage**: Badger for local dev, Postgres for production.
- **Frontend**: Next.js App Router with TailwindCSS.
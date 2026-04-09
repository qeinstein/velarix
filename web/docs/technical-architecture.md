---
title: "Technical Architecture"
description: "Velarix runtime and deployment architecture."
order: 3
---

# Technical Architecture

Velarix combines a symbolic reasoning core with retrieval-aware runtime services.

## Core Components

1. **HTTP API**: Public `/v1` surface for facts, decisions, explanations, slices, reviews, and execution checks.
2. **Reasoning Engine**: Symbolic fact graph with OR-of-AND justifications, negated dependencies, invalidation, and explanation support.
3. **Storage Layer**: Badger for local development, Postgres for production, Redis for shared coordination.
4. **SDKs And Integrations**: Python SDK plus OpenAI, LangGraph, CrewAI, LangChain, and LlamaIndex surfaces.

## Data Flow

When an agent approaches a real action:

1. facts are observed or derived
2. a decision is created from the supporting facts
3. `execute-check` validates that the decision is still supported
4. execution proceeds only while the decision remains valid
5. explanations and lineage remain available for blocked outcomes

## Infrastructure

- **Backend**: Go API and symbolic engine
- **Primary production store**: Postgres
- **Coordination layer**: Redis
- **Console**: Next.js frontend using cookie-based auth

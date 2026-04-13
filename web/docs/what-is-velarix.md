---
title: "What Is Velarix"
description: "Understand what Velarix actually is, the problem it solves, how it fits into an AI system, and what it does not try to be."
section: "Getting Started"
sectionOrder: 1
order: 1
---

# What Velarix is

Velarix is a standalone HTTP service that keeps a session-scoped graph of facts and derived conclusions, then blocks execution when those conclusions are no longer justified.

The core engine is symbolic. Facts are linked by explicit justification sets. A derived conclusion remains valid only while at least one of its justification sets remains satisfied. No LLM is required on the hot path for assertion, invalidation, propagation, explanation, consistency checking, or decision execution.

If you are integrating it into an agent or approval workflow, the mental model is:

1. assert the facts you currently believe
2. derive the conclusion or recommendation from those facts
3. create a first-class decision record
4. run `execute-check` immediately before the side effect
5. execute only if the decision is still supported

## What problem it solves

Prompt context goes stale. An agent can receive a new fact, but still act on reasoning built from an older premise. Velarix is the layer that tracks those dependencies explicitly and invalidates downstream conclusions when support changes.

The useful outcome is not "more memory". The useful outcome is *correctable state*.

## What it is not

Velarix is not:

- an LLM wrapper
- a prompt-engineering library
- a vector database
- a general-purpose agent framework
- a hosted compliance product

It can use an LLM for optional extract-and-assert and optional verifier-assisted contradiction checks, but the reasoning engine itself is not model-driven.

## Runtime shape

From the codebase, Velarix currently ships as:

- a Go HTTP API
- a symbolic reasoning engine in `core/`
- session persistence in Badger for local mode or Postgres for shared deployments
- optional Redis coordination for idempotency and rate limiting
- a Python SDK plus OpenAI, LangChain, LangGraph, CrewAI, and LlamaIndex integrations
- a `vlx` CLI

## A minimal example

```bash
export VELARIX_ENV=dev
export VELARIX_BADGER_PATH="$(mktemp -d)"
go run main.go --lite
```

```bash
curl -sS -X POST http://localhost:8080/v1/s/demo/facts \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "vendor_verified",
    "is_root": true,
    "manual_status": 1.0,
    "payload": {
      "summary": "Vendor 17 passed KYB"
    }
  }'
```

```bash
curl -sS -X POST http://localhost:8080/v1/s/demo/facts \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "decision.release_payment",
    "is_root": false,
    "justification_sets": [["vendor_verified"]],
    "payload": {
      "summary": "Release payment for invoice inv-1042"
    }
  }'
```

That second fact only remains valid while `vendor_verified` remains valid.

## When to use it

Velarix fits best when an AI system is close to a real side effect:

- approvals
- payments
- access changes
- policy decisions
- operational actions that need an auditable why-chain

If a workflow can create audit exposure, move money, or change access, Velarix should sit between the reasoning step and the final action.

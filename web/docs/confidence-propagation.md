---
title: "Confidence Propagation"
description: "See how a root fact change moves through the graph, how the propagation queue works, and why the threshold is central to runtime behavior."
section: "Core Concepts"
sectionOrder: 2
order: 3
---

# Propagation is incremental

When a root fact changes, Velarix does not rebuild the whole graph. The engine enqueues the changed fact and propagates status updates forward through dependent justification sets and child facts.

The implementation lives in `core/engine.go` in `propagate`.

## The queue

The propagation loop maintains:

- a FIFO queue of fact IDs
- a deduplication map so the same fact is not enqueued repeatedly

That matters for diamond-shaped graphs where one change might otherwise cause redundant work.

## Update rules

For each queued fact:

1. compute its new status
2. if the status changed, notify listeners
3. recalculate every dependent justification set
4. enqueue child facts whose support changed

## How AND and OR behave

For one justification set:

- every required parent must be satisfied
- the set confidence is the minimum dependency confidence

For one derived fact:

- evaluate every justification set
- take the maximum confidence across the valid sets

## Short-circuits used by the engine

The propagation loop includes a few practical optimizations:

- queue deduplication
- OR short-circuit when a fact reaches confidence `1.0`
- AND short-circuit for confidence accumulation once the minimum hits `0.0`
- reverse indexes so child justification sets are found directly

These are performance details, but they matter when sessions get large.

## A concrete example

Suppose:

- `vendor_verified = 1.0`
- `invoice_approved = 1.0`
- `decision.release_payment <- ["vendor_verified", "invoice_approved"]`

Now invalidate `vendor_verified`.

Propagation does this:

1. set `vendor_verified.manual_status` to `0`
2. enqueue `vendor_verified`
3. recalculate the child justification set for `decision.release_payment`
4. mark that set invalid because one parent no longer satisfies the threshold
5. enqueue `decision.release_payment`
6. recompute the derived fact status as `0`

## Why `resolved_status` matters

Direct engine support is not the whole story. `GetStatus` also considers:

- retractions
- collapsed roots
- dominator ancestry

That is why API callers should use `resolved_status`, not just raw `derived_status`.

## Example

```bash
curl -sS -X POST http://localhost:8080/v1/s/demo/facts/vendor_verified/invalidate
curl -sS http://localhost:8080/v1/s/demo/facts/decision.release_payment
```

After propagation, the child fact's `resolved_status` should reflect the invalidated dependency chain.

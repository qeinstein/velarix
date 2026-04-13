---
title: "Facts And Belief Graph"
description: "Understand `Fact`, root versus derived facts, `resolved_status`, and how belief state is represented in a session."
section: "Core Concepts"
sectionOrder: 2
order: 1
---

# Facts are the unit of state

Velarix stores everything as `Fact` records. The `Fact` struct in `core/fact.go` is the canonical representation.

## Root facts

A root fact has:

- `is_root: true`
- `manual_status`
- no required `justification_sets`

Example:

```json
{
  "id": "vendor_verified",
  "is_root": true,
  "manual_status": 1.0,
  "payload": {
    "summary": "Vendor 17 passed KYB"
  }
}
```

## Derived facts

A derived fact has:

- `is_root: false`
- one or more `justification_sets`
- `derived_status` computed by the engine

Example:

```json
{
  "id": "decision.release_payment",
  "is_root": false,
  "justification_sets": [["vendor_verified", "invoice_approved"]],
  "payload": {
    "summary": "Release payment for inv-1042"
  }
}
```

## Status fields

From the struct and engine behavior:

- `manual_status`: the root fact's asserted confidence
- `derived_status`: the best currently valid justification confidence for a derived fact
- `resolved_status`: the API-facing effective status after dominator pruning and invalidation effects
- `valid_justification_count`: how many justification sets are currently above the confidence threshold

In practice, treat `resolved_status >= 0.6` as "usable now".

## The confidence threshold

`core.ConfidenceThreshold` is `0.6`.

That threshold drives:

- whether a parent counts as satisfied
- whether a justification set counts as valid
- whether a fact appears valid in slices and consistency checks
- whether a negated dependency is considered satisfied

## Metadata and payload

`payload` is user-facing fact content. `metadata` is internal or system-oriented context.

The server and SDK use metadata for things like:

- provenance
- review state
- source references
- modality, provider, and model for perceptions
- governance controls such as `requires_human_review`

## Embeddings

Facts can store an `embedding []float64`, but the engine can also derive a lexical embedding on demand from the fact's ID, payload, and selected metadata fields.

That is why semantic search works even when no embedding is supplied.

## A concrete inspection example

```bash
curl -sS http://localhost:8080/v1/s/payment-demo/facts/decision.release_payment
```

A typical response includes fields like:

```json
{
  "id": "decision.release_payment",
  "payload": {
    "summary": "Release payment for inv-1042"
  },
  "is_root": false,
  "derived_status": 1,
  "resolved_status": 1,
  "valid_justification_count": 1,
  "justification_sets": [["vendor_verified", "invoice_approved"]]
}
```

If one parent is invalidated, `resolved_status` drops below `0.6` and the fact stops being usable for execution-critical checks.

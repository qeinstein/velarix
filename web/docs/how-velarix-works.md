---
title: "How Velarix Works"
description: "Learn the reasoning model behind facts, justification sets, confidence propagation, contradiction detection, and the explain API."
section: "Getting Started"
sectionOrder: 1
order: 2
---

# The reasoning model

Velarix keeps a graph of `Fact` records. Root facts are asserted directly. Derived facts reference other facts through justification sets.

The important pieces from `core/` are:

- `Fact`
- `JustificationSet`
- `ConfidenceThreshold`
- `ConsistencyReport`
- `ExplanationOutput`

## Facts

A fact has:

- an `id`
- a `payload`
- optional `metadata`
- `is_root`
- either `manual_status` for root facts or `derived_status` for derived facts
- `resolved_status`, which is what callers should use when deciding whether a fact is usable

`ConfidenceThreshold` is `0.6`. Anything below that is treated as not currently valid.

## OR-of-AND justification sets

Derived facts use `justification_sets [][]string`.

Each inner array is one AND clause. The outer array is OR across those clauses.

```json
{
  "id": "decision.release_payment",
  "is_root": false,
  "justification_sets": [
    ["vendor_verified", "invoice_approved"],
    ["vendor_verified", "manual_override_approved"]
  ]
}
```

That means:

- path 1: `vendor_verified AND invoice_approved`
- path 2: `vendor_verified AND manual_override_approved`

If either path is fully satisfied, the derived fact stays valid.

## Negated dependencies

Velarix supports negated parents by prefixing a dependency token with `!`.

```json
{
  "id": "decision.ship_order",
  "is_root": false,
  "justification_sets": [
    ["payment_captured", "!fraud_hold_present"]
  ]
}
```

The derived fact stays supported only while `fraud_hold_present` remains below the confidence threshold.

## Confidence propagation

Each justification set tracks:

- `target_valid_parents`
- `current_valid_parents`
- `confidence`

For a justification set to count as valid, all of its required parents must be satisfied. Its confidence becomes the minimum confidence across the satisfied parents. A derived fact then takes the maximum confidence across its valid justification sets.

In short:

- AND uses `min`
- OR uses `max`

## Contradiction detection

The core symbolic checker looks for five classes of issues:

1. explicit contradiction via the `contradicts` field
2. `claim_key` with different `claim_value`
3. same `subject` and `predicate`, different `object`
4. same `subject`, `predicate`, and `object`, opposite `polarity`
5. optional semantic negation conflict for semantically close facts that appear to negate each other

When `OPENAI_API_KEY` and `VELARIX_VERIFIER_MODEL` are set, the API can also enrich the report with verifier-backed `model_verifier_contradiction` issues.

## Explanations

`ExplainReasoning` returns `ExplanationOutput`, which includes:

- `fact_id`
- `session_id`
- `timestamp`
- `summary`
- `structured`
- `invalidated_fact_ids`
- `sources`
- `policy_versions`
- `causal_chain`
- optional `counterfactual`

The causal chain is a flat list of `BeliefExplanation` records. For negated dependencies, the explanation currently keeps the raw `!fact_id` token in `parents`. There is no separate `NegatedParents` field in the current codebase.

## A concrete walk-through

```bash
curl -sS -X POST http://localhost:8080/v1/s/demo/facts \
  -H 'Content-Type: application/json' \
  -d '{"id":"invoice_approved","is_root":true,"manual_status":1.0,"payload":{"summary":"Invoice approved"}}'
```

```bash
curl -sS -X POST http://localhost:8080/v1/s/demo/facts \
  -H 'Content-Type: application/json' \
  -d '{
    "id":"decision.release_payment",
    "is_root":false,
    "justification_sets":[["vendor_verified","invoice_approved"]],
    "payload":{"summary":"Release payment"}
  }'
```

```bash
curl -sS -X POST http://localhost:8080/v1/s/demo/facts/vendor_verified/invalidate
```

After that invalidation:

- `vendor_verified` drops to `0`
- its dependent justification set drops to `0`
- `decision.release_payment` loses support
- `GET /v1/s/demo/explain?fact_id=decision.release_payment` reports the stale dependency chain

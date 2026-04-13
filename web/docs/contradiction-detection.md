---
title: "Contradiction Detection"
description: "Understand the symbolic contradiction rules, the optional verifier-assisted pass, and what auto-retraction actually does."
section: "Core Concepts"
sectionOrder: 2
order: 4
---

# Symbolic consistency checks

Velarix first checks contradictions symbolically in `core/consistency.go`. That logic does not depend on an LLM.

## The five rule types

### 1. `explicit_contradiction`

If one fact lists the other in `payload.contradicts` or `metadata.contradicts`, the pair is contradictory.

### 2. `claim_value_conflict`

If both facts share `claim_key` but assert different `claim_value`, the pair is contradictory.

### 3. `predicate_object_conflict`

If both facts share `subject` and `predicate` but disagree on `object`, the pair is contradictory.

### 4. `polarity_conflict`

If both facts share `subject`, `predicate`, and `object` but use opposite `polarity`, the pair is contradictory.

### 5. `semantic_negation_conflict`

If two facts are semantically close and one appears negated while the other does not, Velarix can raise a medium-severity semantic negation issue.

The built-in check looks for negation words and a cosine similarity of at least `0.92`.

## Optional verifier-assisted contradictions

If both of these env vars are set:

- `OPENAI_API_KEY`
- `VELARIX_VERIFIER_MODEL`

the API adds a second pass in `api/consistency_verifier.go`.

That verifier:

- builds candidate pairs from currently valid facts
- ranks them by similarity
- sends a small JSON classification prompt to an OpenAI-compatible chat API
- appends `model_verifier_contradiction` issues when the model labels a pair as contradictory

This pass is optional and additive. It does not replace the symbolic rules.

## What happens when a contradiction is found

By default, Velarix reports the issue. It does not retract facts automatically unless one of these is true:

- `auto_retract_contradictions` is `true` on `extract-and-assert`
- session config sets `auto_retract_contradictions: true`
- a reasoning-chain verification request uses `auto_retract: true`

When auto-retraction is active, Velarix retracts the lower-entrenchment fact in each contradictory pair.

## Example

```bash
curl -sS -X POST http://localhost:8080/v1/s/demo/facts \
  -H 'Content-Type: application/json' \
  -d '{
    "id":"invoice_status_paid",
    "is_root":true,
    "manual_status":1.0,
    "payload":{"claim_key":"invoice_status","claim_value":"paid"}
  }'
```

```bash
curl -sS -X POST http://localhost:8080/v1/s/demo/facts \
  -H 'Content-Type: application/json' \
  -d '{
    "id":"invoice_status_unpaid",
    "is_root":true,
    "manual_status":1.0,
    "payload":{"claim_key":"invoice_status","claim_value":"unpaid"}
  }'
```

```bash
curl -sS -X POST http://localhost:8080/v1/s/demo/consistency-check \
  -H 'Content-Type: application/json' \
  -d '{
    "fact_ids":["invoice_status_paid","invoice_status_unpaid"],
    "include_invalid": false
  }'
```

The report should include a `claim_value_conflict`.

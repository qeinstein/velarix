---
title: "Justification Sets"
description: "See exactly how Velarix models OR-of-AND support, including negated dependencies, validation rules, and failure modes."
section: "Core Concepts"
sectionOrder: 2
order: 2
---

# OR-of-AND support

`Fact.JustificationSets` is `[][]string`.

Velarix interprets it as OR across inner arrays and AND within each inner array.

## One AND clause

```json
[["vendor_verified", "invoice_approved"]]
```

This says the child fact is valid only if both parents are satisfied.

## Two alternative support paths

```json
[
  ["vendor_verified", "invoice_approved"],
  ["manual_override_approved"]
]
```

This says the child fact stays valid if either:

- `vendor_verified AND invoice_approved`
- `manual_override_approved`

## Negated dependencies

Prefix a dependency with `!` to require the parent to stay below the confidence threshold.

```json
[
  ["payment_captured", "!fraud_hold_present"]
]
```

Internally, `splitDependencySet` turns this into:

- `positive_parent_fact_ids`
- `negative_parent_fact_ids`
- `parent_fact_ids`

## Validation rules

From `assertFactInner` and dependency parsing:

- a non-root fact must have at least one justification set
- a justification set cannot be empty
- every dependency token must contain a fact ID
- every referenced parent fact must already exist
- adding the fact must not create a cycle

## What happens when one set is invalid

Only the invalid set drops out. The child fact remains valid if at least one other set is still valid.

Example:

```json
[
  ["vendor_verified", "invoice_approved"],
  ["finance_director_waiver"]
]
```

If `invoice_approved` becomes invalid:

- set 1 becomes invalid
- set 2 can still keep the child fact valid
- `valid_justification_count` becomes `1`

## What happens when all sets are invalid

The child fact's `derived_status` becomes `0`, and its `resolved_status` also becomes unusable.

## Example request

```bash
curl -sS -X POST http://localhost:8080/v1/s/demo/facts \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "decision.ship_order",
    "is_root": false,
    "justification_sets": [
      ["payment_captured", "!fraud_hold_present"],
      ["manual_release_approved"]
    ],
    "payload": {
      "summary": "Ship the order"
    }
  }'
```

This is the exact shape the API expects. The SDK uses the same nested list structure.

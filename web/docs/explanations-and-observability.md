---
title: "Explanations And Observability"
description: "Inspect explanation payloads, understand the causal chain and counterfactual output, and monitor a Velarix deployment with the built-in Prometheus metrics."
section: "Operations"
sectionOrder: 6
order: 2
---

# Explanations and observability

This page covers two related areas:

- what the explanation system returns
- what the server exports for monitoring

## `ExplanationOutput`

The explanation payload in `core/explain.go` contains:

- `fact_id`
- `session_id`
- `timestamp`
- `summary`
- `structured`
- `invalidated_fact_ids`
- `sources`
- `policy_versions`
- `causal_chain`
- `counterfactual`

## `structured`

`structured` is a list of `BeliefExplanation` records. Each record contains:

- `fact_id`
- `confidence`
- `tier`
- `provenance`
- `payload`
- `is_root`
- `parents`

This is the detailed explanation tree. It is where you inspect which facts contributed to the conclusion and with what confidence.

## `causal_chain`

`causal_chain` is the ordered sequence of fact IDs that the explainer identifies as the main path supporting the requested fact. It is a flattened trace, not a full graph export.

Use it when you need a compact answer to "which beliefs mattered most here?" rather than the entire supporting tree.

## `invalidated_fact_ids`

These are fact IDs that appear in the explanation context but are currently invalidated or retracted enough to matter to the narrative.

## `sources`

Each `ExplanationSource` record contains:

- `fact_id`
- `source_type`
- `source_ref`
- `payload_hash`
- `policy_version`

These fields let you tie the explanation back to source provenance and policy versions without reading the entire fact payload.

## Counterfactual analysis

If `counterfactual_fact_id` is supplied to `/explain`, the response includes a `counterfactual` object with:

- `removed_fact_id`
- `impacted_facts`
- `direct_count`
- `total_count`
- `epistemic_loss`
- `narrative`

This answers the question: "What downstream beliefs would weaken or disappear if this fact were removed?"

Example:

```bash
curl -sS "http://localhost:8080/v1/s/demo/explain?fact_id=decision.release_payment&counterfactual_fact_id=vendor_verified" \
  -H "Authorization: Bearer $VELARIX_API_KEY"
```

## One correction worth stating explicitly

There is no `NegatedParents` field in `ExplanationOutput`. Negated dependencies are represented in justification sets with `!fact_id` tokens and are resolved by the dependency parser and justification validator. If you need to explain a negated dependency, document the `!` syntax, not a nonexistent response field.

## Persisted explanation records

Explanations and reasoning artifacts are stored as immutable explanation records with:

- `timestamp`
- `content_hash`
- `tampered`
- raw `content`

The store computes hashes so later readers can detect tampering.

## Prometheus metrics

Defined in `api/metrics.go`:

- `velarix_active_sessions_total`
  - current number of sessions loaded in RAM
- `velarix_extraction_latency_ms`
  - end-to-end extraction latency in milliseconds
- `velarix_verifier_failures_total{reason=...}`
  - consistency-verifier failures by reason
- `velarix_auto_retractions_total`
  - facts automatically retracted because of contradictions
- `velarix_badger_disk_usage_bytes`
  - local Badger usage
- `velarix_fact_assertion_latency_ms`
  - assertion latency
- `velarix_api_requests_total{endpoint,status}`
  - request counts by endpoint and status
- `velarix_cache_ratio{type}`
  - slice cache hits and misses
- `velarix_prune_latency_ms`
  - causal collapse and pruning latency
- `velarix_slo_success_rate{result}`
  - coarse success-rate counter

## Access control for `/metrics`

The metrics middleware restricts `/metrics` by CIDR using `VELARIX_METRICS_ALLOWED_CIDR`. If unset, the code defaults to localhost-oriented access only. Do not expose this route publicly unless you intend to.

## What to watch first

- `velarix_api_requests_total` for error rates
- `velarix_fact_assertion_latency_ms` for write latency
- `velarix_extraction_latency_ms` if you use extract-and-assert
- `velarix_auto_retractions_total` for contradiction churn
- `velarix_active_sessions_total` for memory pressure

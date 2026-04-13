---
title: "API: Reasoning, Consistency, And Extract"
description: "Reference the explanation, consistency, reasoning-chain, perception, and extract-and-assert endpoints, including rate limits, payload shapes, and response contracts."
section: "API Reference"
sectionOrder: 3
order: 3
---

# Reasoning and consistency endpoints

These routes all live under `/v1/s/{session_id}`. In full mode they require authentication. In Lite mode they remain available, but the org, account, and admin routes are not registered.

## `GET /v1/s/{session_id}/explain`

Build a structured explanation for the current belief state, or for a historical point reconstructed from the session journal.

Query parameters:

- `fact_id` (`string`, optional): focus the explanation on one fact
- `timestamp` (`string`, optional): rebuild the session using only journal entries at or before this millisecond timestamp
- `counterfactual_fact_id` (`string`, optional): include counterfactual impact analysis for removing one fact

Response body:

- `fact_id` (`string`)
- `session_id` (`string`)
- `timestamp` (`int64`)
- `summary` (`string`)
- `structured` (`BeliefExplanation[]`)
- `invalidated_fact_ids` (`string[]`)
- `sources` (`ExplanationSource[]`)
- `policy_versions` (`string[]`)
- `causal_chain` (`string[]`)
- `counterfactual` (`CounterfactualResult`, optional)

`BeliefExplanation` entries contain:

- `fact_id`
- `confidence`
- `tier`
- `provenance`
- `payload`
- `is_root`
- `parents`

`CounterfactualResult` contains:

- `removed_fact_id`
- `impacted_facts`
- `direct_count`
- `total_count`
- `epistemic_loss`
- `narrative`

`ExplanationSource` contains:

- `fact_id`
- `source_type`
- `source_ref`
- `payload_hash`
- `policy_version`

Example:

```bash
curl -sS "http://localhost:8080/v1/s/demo/explain?fact_id=decision.release_payment&counterfactual_fact_id=vendor_verified" \
  -H "Authorization: Bearer $VELARIX_API_KEY"
```

## `GET /v1/s/{session_id}/explanations`

List persisted explanation records for the session.

Response body:

- array of explanation records from the store
- each record includes `timestamp`, `content_hash`, `tampered`, and the original explanation content

Use this when you need the immutable history of explanations, not just a freshly generated one.

## `GET /v1/s/{session_id}/facts/{id}/why`

Legacy explanation endpoint for one fact.

Response body:

- explanation payload for the requested fact

Prefer `GET /explain` for new integrations because it supports historical reconstruction and counterfactual analysis.

## `POST /v1/s/{session_id}/consistency-check`

Run the contradiction detector over the supplied facts, or over the entire session if `fact_ids` is omitted.

Request body:

- `fact_ids` (`string[]`, optional): explicit fact subset
- `max_facts` (`integer`, optional): truncates the check set after sorting
- `include_invalid` (`boolean`, optional): include facts already below the confidence threshold

Response body:

- `checked_fact_ids` (`string[]`)
- `issue_count` (`int`)
- `issues` (`ConsistencyIssue[]`)

Each `ConsistencyIssue` contains:

- `rule` (`string`)
- `fact_ids` (`string[]`)
- `message` (`string`)
- `severity` (`string`)

Important limits:

- rate-limited to `5` requests per minute per org
- synchronous checks refuse sessions with more than `10_000` facts and return `422 Unprocessable Entity`
- if session config enables `auto_retract_contradictions`, the lower-entrenchment fact in each conflicting pair is retracted automatically after the report is produced
- if the optional OpenAI-compatible verifier is configured, extra issues with rule `model_verifier_contradiction` may be appended

Error cases:

- `400` invalid JSON
- `403` session belongs to another org
- `422` too many facts for synchronous checking
- `429` org rate limit exceeded

Example:

```bash
curl -sS -X POST http://localhost:8080/v1/s/demo/consistency-check \
  -H "Authorization: Bearer $VELARIX_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "fact_ids":["invoice.paid","invoice.unpaid"],
    "include_invalid":false
  }'
```

## `POST /v1/s/{session_id}/reasoning-chains`

Persist an external reasoning artifact so Velarix can audit it later.

Request body:

- `chain_id` (`string`, optional): auto-generated as `rc_<timestamp>` when omitted
- `model` (`string`, optional)
- `mode` (`string`, optional)
- `summary` (`string`, optional)
- `created_at` (`int64`, optional): current time is used when omitted
- `steps` (`ReasoningStep[]`, required)

Each `ReasoningStep` contains:

- `id` (`string`, optional): auto-generated per step when omitted
- `kind` (`string`, optional)
- `content` (`string`, required)
- `evidence_fact_ids` (`string[]`, optional)
- `justification_fact_ids` (`string[]`, optional)
- `output_fact_id` (`string`, optional)
- `contradicts_fact_ids` (`string[]`, optional)
- `confidence` (`number`, optional)

Response body:

- the stored reasoning chain

Example:

```bash
curl -sS -X POST http://localhost:8080/v1/s/demo/reasoning-chains \
  -H "Authorization: Bearer $VELARIX_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "model":"gpt-4.1",
    "mode":"analysis",
    "summary":"Check whether payment can be released",
    "steps":[
      {
        "content":"Vendor verification and invoice approval support payment release.",
        "evidence_fact_ids":["vendor_verified","invoice_approved"],
        "output_fact_id":"decision.release_payment",
        "confidence":0.88
      }
    ]
  }'
```

## `GET /v1/s/{session_id}/reasoning-chains`

List stored reasoning-chain records for the session.

Response body:

- `{ "items": [...] }`

Each item contains:

- `chain`
- `timestamp`
- `content_hash`
- `tampered`

## `POST /v1/s/{session_id}/reasoning-chains/{chain_id}/verify`

Audit a stored chain against the current session state.

Request body:

- `auto_retract` (`boolean`, optional): retract candidate facts if verification fails

Response body:

- `chain_id`
- `valid`
- `summary`
- `step_audits`
- `issues`
- `retract_candidate_fact_ids`
- `auto_retracted_fact_ids`
- `verified_at`

Each `ReasoningStepAudit` contains:

- `step_id`
- `valid`
- `missing_fact_ids`
- `invalid_fact_ids`
- `output_fact_id`
- `consistency_findings`

Important behavior:

- shares the same `5` requests per minute org-level limiter as `consistency-check`
- audits missing, invalid, and contradictory evidence
- if a step outputs a fact, the verifier also checks that output against prior outputs in the chain

Error cases:

- `403` unauthorized session
- `404` reasoning chain not found
- `429` rate limit exceeded

## `POST /v1/s/{session_id}/percepts`

Record a root fact whose provenance is a perception or model output.

Request body:

- `id` (`string`, required)
- `payload` (`object`, required)
- `confidence` (`number`, optional): defaults to `0.75`
- `modality` (`string`, optional)
- `provider` (`string`, optional)
- `model` (`string`, optional)
- `embedding` (`number[]`, optional)
- `metadata` (`object`, optional)

Response body:

- the stored fact, with `_provenance` moved from payload into metadata if present

The handler always stamps `metadata.source_type = "perception"`.

## `POST /v1/s/{session_id}/extract-and-assert`

Extract facts from LLM output, assert every valid extracted fact, and optionally auto-retract contradictions among the newly asserted facts.

Request body:

- `llm_output` (`string`, required, max `32000` characters)
- `session_context` (`string`, optional, max `2000` characters)
- `auto_retract_contradictions` (`boolean`, optional)

Response body:

- `extracted_count`
- `asserted_count`
- `skipped_count`
- `contradictions_found`
- `contradictions_retracted`
- `facts`

Important behavior:

- extracted facts are sorted so all roots are asserted before any derived fact that depends on them
- assertion failures, cycles, and invalid dependencies are skipped instead of aborting the entire request
- if journal persistence fails after an assertion, the in-memory fact is rolled back so engine state and history stay aligned
- contradiction auto-retraction compares effective entrenchment and retracts the lower-entrenchment fact

Error cases:

- `400` invalid body, missing `llm_output`, or overlong fields
- `403` unauthorized session
- `502` extraction backend failure

Example:

```bash
curl -sS -X POST http://localhost:8080/v1/s/demo/extract-and-assert \
  -H "Authorization: Bearer $VELARIX_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "llm_output":"The vendor is verified. The invoice is approved. Release the payment.",
    "session_context":"We are evaluating invoice inv-1042 for vendor-17.",
    "auto_retract_contradictions":true
  }'
```

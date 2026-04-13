---
title: "Benchmark"
description: "Run the built-in benchmark harness, understand the datasets it uses, and interpret the JSON and Markdown reports it produces."
section: "Benchmark"
sectionOrder: 7
order: 1
---

# Benchmark harness

Velarix includes a benchmark harness in `benchmark/harness.go` plus dataset loaders in `benchmark/datasets.go`. The goal is to evaluate contradiction handling and grounding behavior, not raw language-model quality in the abstract.

## What the harness does

The harness:

- loads TruthfulQA and HaluEval data
- queries a baseline OpenAI-compatible model
- optionally routes outputs through Velarix by calling `/v1/s/{session_id}/extract-and-assert`
- reconstructs a grounded answer by matching generated sentences to currently valid extracted facts
- writes `report.json` and `report.md`

The Velarix path is not just "call the same model with a different prompt". It explicitly extracts facts, asserts them, detects contradictions, and then rebuilds an answer from facts that survived that process.

## `benchmark.Config`

Fields in `benchmark/harness.go`:

- `baseline_model`
- `velarix_url`
- `benchmark_dataset_path`
- `output_path`
- `temperature`
- `max_tokens`
- `auto_retract_contradictions`
- `runs_per_question`
- `hedge_string`
- `openai_api_key`
- `openai_base_url`

Constraint:

- `temperature` must be `0.0`

## Datasets

### TruthfulQA

Used to measure whether a system produces answers aligned with factual reality instead of common misconceptions.

### HaluEval

Used to measure hallucination behavior and unsupported generation.

`benchmark/datasets.go` downloads both datasets automatically if they are not already cached locally.

## Running the Go harness

Run the server first, then run the benchmark command or integrate the harness in Go code.

Example CLI flow:

```bash
vlx benchmark --steps 120 --contradiction-interval 17 --output benchmark.json
```

The CLI command actually delegates to `tests/reproducibility/hallucination_benchmark.py`, which is the reproducibility harness currently wired into the user-facing tool.

## Running the HTTP-backed benchmark path

When the harness uses Velarix, it posts extracted content to:

- `POST /v1/s/{session_id}/extract-and-assert`

If `auto_retract_contradictions` is enabled, contradictions found among newly extracted facts may be retracted immediately.

## Output files

### `report.json`

Machine-readable benchmark results.

### `report.md`

Human-readable summary suitable for inspection or review.

## How grounding reconstruction works

After fact extraction, the harness compares generated sentences against valid extracted facts and keeps sentences whose similarity score is at least `0.75`. That makes the reported grounded output dependent on the surviving fact set, not only on the raw generation.

## How to interpret results

- better contradiction handling means fewer mutually inconsistent facts survive
- better grounding means more of the final answer can be reconstructed from valid asserted facts
- higher extraction counts are not automatically better if many facts are skipped or retracted
- compare baseline and Velarix runs on the same model, temperature, token budget, and dataset split

## Practical limitation

The benchmark measures the combined effect of extraction quality, contradiction detection, and fact-grounded reconstruction. It does not isolate those stages unless you instrument the harness further.

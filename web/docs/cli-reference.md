---
title: "CLI Reference"
description: "Use the `vlx` CLI to check server health, inspect slices, review facts, mutate facts, export compliance data, and run the benchmark harness."
section: "CLI"
sectionOrder: 5
order: 1
---

# `vlx` CLI

The CLI entrypoint is `cmd/vlx/main.go`. It is a thin HTTP client plus a wrapper around the reproducibility benchmark script.

## Environment variables

- `VELARIX_BASE_URL`: base URL used when `--url` is not provided
- `VELARIX_API_KEY`: bearer token for authenticated commands
- `VELARIX_BENCHMARK_BINARY`: mentioned in the usage text for spawned benchmark runs

Code detail worth noting: the compiled constant in the source is `https://localhost:8080`, while the usage text still says `http://localhost:8080`. The code wins.

## `vlx status`

Check service health.

Flags:

- `--url`: override base URL
- `--full`: call `/health/full` instead of `/health`

Example:

```bash
vlx status --url http://localhost:8080 --full
```

Expected output format:

- pretty-printed JSON from the health endpoint

## `vlx slice`

Fetch a query-aware session slice.

Flags:

- `--url`
- `--session` required
- `--format` default `json`, allowed `json` or `markdown`
- `--query`
- `--strategy` default `hybrid`
- `--max-facts` default `50`
- `--max-chars` default `0`
- `--include-dependencies` default `true`
- `--include-invalid` default `false`

Example:

```bash
vlx slice \
  --url http://localhost:8080 \
  --session demo \
  --format markdown \
  --query "payment release evidence" \
  --max-facts 20
```

Expected output format:

- raw markdown when `--format markdown`
- pretty-printed JSON when `--format json`

## `vlx review`

Set review state on a fact.

Flags:

- `--url`
- `--session` required
- `--fact` required
- `--status` required: `pending`, `approved`, `waived`, or `rejected`
- `--reason`

Example:

```bash
vlx review --session demo --fact vendor_verified --status approved --reason "Reviewed by analyst"
```

Expected output format:

- pretty-printed JSON fact record

## `vlx invalidate`

Invalidate a root fact.

Flags:

- `--url`
- `--session` required
- `--fact` required
- `--reason`
- `--force` admin-only governance override

Example:

```bash
vlx invalidate --session demo --fact vendor_verified --reason "Vendor registration expired"
```

## `vlx retract`

Retract a fact.

Flags:

- `--url`
- `--session` required
- `--fact` required
- `--reason`
- `--force`

Example:

```bash
vlx retract --session demo --fact decision.release_payment --reason "Decision superseded"
```

Expected output format for both mutation commands:

- pretty-printed JSON status payload

## `vlx compliance-export`

Export org-level audit data.

Flags:

- `--url`
- `--format` default `json`, allowed `json` or `ndjson`
- `--limit` default `500`
- `--output`: optional file path

Example:

```bash
vlx compliance-export --url http://localhost:8080 --format ndjson --limit 1000 --output compliance.ndjson
```

Expected output format:

- if `--output` is omitted, the command prints the export payload
- if `--output` is set, it writes the file and prints `Wrote <format> export to <path>`

## `vlx benchmark`

Run the Python reproducibility benchmark harness in `tests/reproducibility/hallucination_benchmark.py`.

Flags:

- `--steps` default `120`
- `--contradiction-interval` default `17`
- `--spawn-server`
- `--output`
- `--python` default `python3`

Example:

```bash
vlx benchmark --steps 200 --contradiction-interval 25 --output benchmark.json
```

Expected output format:

- the CLI streams the Python script output directly to stdout and stderr
- exit status comes from the Python harness process

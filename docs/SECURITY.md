# Security Notes

This file describes the security posture of the repository today and the gaps that still remain.

## What Exists Today

- API key authentication for service access
- JWT-based user authentication for console-oriented flows
- org-scoped request handling
- optional Badger encryption at rest, required outside `dev`
- append-only history with verification hashes
- scope-aware route checks

## What Matters For This Product

The real security value of Velarix is not broad compliance marketing.

It is:

- preventing stale approvals from executing
- preserving decision provenance
- enforcing org boundaries

## Important Gaps

- password reset is not a finished production delivery flow
- rate limiting and idempotency still need the shared-store production path to be the default
- retention and deletion are not the same thing as invalidation
- export integrity hashes are not audited compliance evidence

## Practical Guidance

Use this repository as:

- a focused approval-guardrail service under active development

Do not represent it as:

- a finished compliance product
- a healthcare-ready platform
- a multi-region security platform

## Security Rule

If a workflow can move money, change access, or create audit exposure, require a fresh `execute-check` before the action is performed.


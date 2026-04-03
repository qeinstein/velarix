# Security Notes

This file describes the security mechanisms that exist in the repository today and the gaps that still remain.

## Implemented Today

- JWT-based user login for console-oriented flows
- API key authentication for service access
- org-scoped request handling
- optional Badger encryption at rest, required outside `VELARIX_ENV=dev`
- append-only history with verification hashes for export integrity

## Important Gaps

- Password reset currently depends on a server-side logged token and should not be treated as production-ready delivery.
- Rate limiting and idempotency are still tied to the current local store path and are not yet distributed.
- Invalidating a fact is not the same as deleting stored data for retention or legal deletion requirements.
- Export verification hashes improve tamper detection, but they do not imply audited compliance posture.

## Practical Guidance

Use this repository as a decision-integrity service under active development. Do not represent the current implementation as a finished compliance, healthcare, or multi-region security platform.

# Operations

This guide covers the current operational reality of Velarix as an approval-guardrail service.

## Current Deployment Modes

### Local Development

Use Badger for:

- local work
- tests
- demo runs

### Production Direction

Use Postgres plus Redis for:

- shared state
- multi-instance safety
- distributed idempotency
- distributed rate limiting

## Health Endpoints

- `GET /health`: basic connectivity and uptime
- `GET /health/full`: detailed health information for admins

## Key Metrics

Watch these first:

- API request counts and status codes
- active sessions in memory
- fact assertion latency
- invalidation latency
- write backpressure events
- execute-check and blocked execute volume

## Backpressure

Velarix uses per-org write backpressure to protect latency under bursty write load.

When saturated, write routes return:

- `503 Service Unavailable`
- `Retry-After: 1`
- `X-Velarix-Backpressure: 1`

Use idempotency keys for safe retries.

## Rate Limiting

Current behavior:

- local and shared-store code paths exist
- the long-term production path should be Redis-backed

Do not treat local-store rate limiting as the final production design.

## Backups And Restore

Badger local backup and restore exist for the local adapter path.

That is not the final production recovery story.

For the shared-store product direction, the recovery story should center on:

- Postgres backups
- Redis operational recovery
- artifact storage strategy for snapshots and exports

## Retention

Retention settings should be treated as operational policy, not only read filtering.

The product should enforce:

- activity retention
- access-log retention
- notification retention

## Environment Guidance

For production-like work, set:

- `VELARIX_ENV=prod`
- `VELARIX_JWT_SECRET`
- `VELARIX_ALLOWED_ORIGINS`
- `VELARIX_STORE_BACKEND=postgres`
- `VELARIX_POSTGRES_DSN`
- `VELARIX_REDIS_URL`

If using Badger outside development, also set:

- `VELARIX_ENCRYPTION_KEY`

## Operational Rule

Do not market this repo as a finished enterprise control plane until the shared-store path is the default production reality.


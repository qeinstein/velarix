# Operations

Velarix is operated as a decision-integrity service.

## Deployment Modes

### Local Development

Use Badger for:

- local development
- tests
- demos

### Production

Use Postgres for:

- shared state
- durable runtime records
- multi-instance operation

Use Redis for:

- shared rate limiting
- shared idempotency
- coordination across instances

Badger in production is an explicit opt-in path, not the default operating mode.

## Health Endpoints

- `GET /health`: connectivity, version, and uptime
- `GET /health/full`: detailed status for admin operators

## Key Metrics

Track these first:

- API request counts and status codes
- active sessions in memory
- fact assertion latency
- invalidation latency
- write backpressure events
- execute-check volume
- blocked execute volume

## Backpressure

Velarix applies per-org write backpressure to protect latency under bursty write load.

When saturated, write routes return:

- `503 Service Unavailable`
- `Retry-After: 1`
- `X-Velarix-Backpressure: 1`

Use idempotency keys for safe retries.

## Rate Limiting And Idempotency

Velarix supports:

- route-level auth throttling
- API key and JWT rate limiting
- request replay protection with idempotency keys

For multi-instance deployments, Redis is the coordination layer.

## Server Defaults

Outside development, Velarix expects:

- `VELARIX_ENV=prod`
- `VELARIX_JWT_SECRET`
- `VELARIX_ALLOWED_ORIGINS`
- `VELARIX_STORE_BACKEND=postgres`
- `VELARIX_POSTGRES_DSN`

Add `VELARIX_REDIS_URL` for shared coordination.

If Badger is used outside development, also set:

- `VELARIX_ENCRYPTION_KEY`
- `VELARIX_ALLOW_BADGER_PROD=true`

## Recovery

The local adapter supports Badger backup and restore.

The production recovery model centers on:

- Postgres backup and restore
- Redis operational recovery
- export and artifact retention outside the reasoning process

## Operating Rule

Keep bootstrap admin access disabled in production unless it is actively required for recovery.

Require `execute-check` immediately before every material side effect.

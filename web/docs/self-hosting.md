---
title: "Self-Hosting"
description: "Run Velarix in Lite or full mode, configure storage and auth, wire up TLS and Redis, and verify a production deployment before exposing it to other systems."
section: "Operations"
sectionOrder: 6
order: 1
---

# Self-hosting

Velarix is a standalone HTTP service. The core engine is symbolic. LLM-backed extraction and optional semantic contradiction verification are side features, not the main execution path.

## Deployment modes

### Lite mode

Enable with either:

- `velarix --lite`
- `VELARIX_LITE=true`

Lite mode only registers:

- health routes
- `/metrics`
- docs assets
- the session-scoped reasoning API under `/v1/s/...`

Use Lite mode for local development, embedded sidecars, tests, and single-tenant setups where you do not need account management, API keys, org-level console routes, or billing-like admin features.

### Full mode

Full mode registers auth, org, user, invitation, billing, policy, and admin endpoints in addition to the session API.

## Storage backends

### Badger

Best for:

- local development
- tests
- single-node environments

Relevant variables:

- `VELARIX_STORE_BACKEND=badger`
- `VELARIX_BADGER_PATH` default `./velarix.data`
- `VELARIX_ALLOW_BADGER_PROD=true` to bypass the production safety check

### Postgres

Best for:

- production
- shared service deployments

Relevant variables:

- `VELARIX_STORE_BACKEND=postgres`
- `VELARIX_POSTGRES_DSN`

The Postgres adapter runs embedded SQL migrations on startup.

### Redis

Redis is optional. It does not replace the main store. It is used for:

- idempotency records
- distributed rate limiting

Relevant variables:

- `VELARIX_REDIS_URL`
- `VELARIX_DISABLE_REDIS=true` to force in-process fallbacks

## Environment variable reference

Core server:

- `PORT`: listen port
- `VELARIX_ENV`: environment name used by startup logic and warnings
- `VELARIX_LITE`: enable Lite mode
- `VELARIX_ENCRYPTION_KEY`: store encryption key; Badger requires 16, 24, or 32 bytes when encryption is enabled
- `VELARIX_STORE_BACKEND`: `badger` or `postgres`
- `VELARIX_POSTGRES_DSN`: Postgres DSN
- `VELARIX_BADGER_PATH`: Badger data path
- `VELARIX_ALLOW_BADGER_PROD`: allow Badger in production mode
- `VELARIX_REDIS_URL`: Redis connection string
- `VELARIX_DISABLE_REDIS`: disable Redis-backed idempotency and rate limiting
- `VELARIX_TLS_CERT`: TLS certificate path
- `VELARIX_TLS_KEY`: TLS key path

Authentication and identity:

- `VELARIX_JWT_SECRET`: required for JWT signing in full mode
- `VELARIX_DECISION_TOKEN_SECRET`: required for signing decision execution tokens
- `VELARIX_API_KEY`: optional bootstrap admin API key
- `VELARIX_ADMIN_EMAIL`: bootstrap admin email
- `VELARIX_AUTH_COOKIE_DOMAIN`: auth cookie domain
- `VELARIX_AUTH_COOKIE_SAMESITE`: auth cookie same-site policy
- `VELARIX_BASE_URL`: base URL used in password-reset links and some SDK helpers

Email reset flow:

- `VELARIX_SMTP_ADDR`
- `VELARIX_SMTP_USER`
- `VELARIX_SMTP_PASS`
- `VELARIX_SMTP_FROM`

HTTP protection and limits:

- `VELARIX_ALLOWED_ORIGINS`: explicit CORS allowlist
- `VELARIX_METRICS_ALLOWED_CIDR`: CIDR allowlist for `/metrics`; defaults to localhost-only behavior
- `VELARIX_IDEMPOTENCY_TTL_HOURS`: TTL for idempotency records
- `VELARIX_MAX_CONCURRENT_WRITES`: org-level write concurrency cap

Consistency verifier:

- `OPENAI_API_KEY`
- `VELARIX_VERIFIER_MODEL`
- `VELARIX_OPENAI_BASE_URL`
- `VELARIX_VERIFIER_MAX_PAIRS`

Background and maintenance:

- `VELARIX_RETENTION_SWEEP_INTERVAL_MINUTES`

Billing and optional console integrations:

- `STRIPE_SECRET_KEY`
- `STRIPE_WEBHOOK_SECRET`
- `BILLING_PORT`

Stress tooling:

- `VELARIX_STRESS_WORKERS`
- `VELARIX_STRESS_REQUESTS_PER_WORKER`

## HTTP and TLS behavior

- maximum request body size is `4 MB`
- if `VELARIX_TLS_CERT` and `VELARIX_TLS_KEY` are set, the server listens with TLS
- if they are not set, the server logs a warning and serves plain HTTP

## CORS

Cross-origin requests are only allowed when `VELARIX_ALLOWED_ORIGINS` is configured and the request origin matches one of the configured values. The middleware also sets `Access-Control-Allow-Credentials: true`.

## Docker guidance

The repo already includes container support. The important deployment choice is not the base image, but which mode and backends you wire in.

For a production container:

1. Set `VELARIX_STORE_BACKEND=postgres`
2. Provide `VELARIX_POSTGRES_DSN`
3. Provide `VELARIX_REDIS_URL`
4. Set `VELARIX_JWT_SECRET` and `VELARIX_DECISION_TOKEN_SECRET`
5. Mount TLS cert and key if terminating TLS in-process
6. Persist no Badger volume unless you explicitly want local Badger mode

## Postgres checklist

- create a database and user for Velarix
- supply a valid `VELARIX_POSTGRES_DSN`
- verify startup completes embedded migrations
- confirm session writes, decisions, explanations, and export jobs persist across restarts

## Redis checklist

- supply `VELARIX_REDIS_URL`
- keep Redis reachable from all app instances
- verify idempotent POST replays and rate limits work across instances

## Production checklist

- use full mode unless you explicitly want a Lite-only deployment
- use Postgres, not Badger, for multi-user or production service deployments
- set `VELARIX_JWT_SECRET`
- set `VELARIX_DECISION_TOKEN_SECRET`
- set `VELARIX_ALLOWED_ORIGINS`
- set `VELARIX_METRICS_ALLOWED_CIDR`
- terminate TLS either at the Velarix process or upstream
- confirm `/health/full` passes
- confirm `/metrics` is reachable only from your metrics network
- confirm Redis-backed idempotency and rate limiting if running multiple instances

## Limitation to state plainly

The repo does not define a single canonical production topology. It provides the server, backends, and env switches, but load balancer layout, secret delivery, backup policy, and HA topology are deployment decisions you still need to make.

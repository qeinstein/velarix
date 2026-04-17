# Velarix Pre-Deployment Audit — Remaining Items

Items resolved by the v0.7.0 hardening pass are removed. 46 items remain.

---

## Tier 1 — Blockers (do not ship without these)

| # | Area | Item |
|---|------|------|
| 1 | Payments | Stripe checkout session creation + webhook handler not wired to frontend — no way to upgrade a plan |
| 2 | Auth | Email verification flow exists in backend but no frontend page for `/verify?token=…` |
| 3 | Auth | Password reset: backend endpoints exist, frontend form missing |
| 5 | Infra | GCS backup for facts/sessions — Cloud Run filesystem is ephemeral; current local-write code loses data on restart |
| 7 | Infra | No readiness probe — Cloud Run sends traffic before the spaCy model finishes loading (add `GET /readyz` that returns 503 until model warm) |

---

## Tier 2 — Must fix before beta

| # | Area | Item |
|---|------|------|
| 11 | Frontend | Session viewer: no fact detail panel (click a node → see full subject/predicate/object/confidence/depends_on) |
| 12 | Frontend | Session viewer: edge labels not rendered (relationship type invisible on graph) |
| 13 | Frontend | Session viewer: no filter by confidence, type, or retraction status |
| 14 | Frontend | No assert/retract from UI — users must use raw API |
| 15 | Frontend | Decision list page missing; execute-check UI missing |
| 16 | Frontend | No error boundary — any React render error crashes the whole app |
| 17 | Frontend | No 404 or 500 pages |
| 18 | Frontend | No toast/notification system — operations succeed or fail silently |
| 19 | Frontend | All buttons lack loading state — double-submit possible on slow connections |
| 20 | Frontend | `localStorage.removeItem("token")` dead code in logout — auth uses httpOnly cookie, so this never clears the real session |
| 27 | Backend | Session config stored in-memory only — lost on restart, not reloaded from DB on next cold start for that session |
| 28 | Frontend | `NEXT_PUBLIC_VELARIX_API_URL` defaults to `localhost:8080` — Next.js bakes public env vars at build time; production build will silently point at localhost |
| 29 | Frontend | No `.env.example` in `web/` — contributors and deployment have no reference |
| 30 | Frontend | No `middleware.ts` protecting dashboard routes — auth check is inside components; HTML shell loads before redirect fires |
| 31 | Frontend | `credentials: "include"` on all fetches requires explicit CORS `Allow-Credentials` + origin matching on the backend |
| 32 | Backend | No webhook signature on outbound webhooks — webhook receivers cannot verify the request came from Velarix |

---

## Tier 3 — Before charging / public launch

| # | Area | Item |
|---|------|------|
| 33 | Payments | Billing portal link (Stripe customer portal) not wired — users can't manage subscription |
| 34 | Compliance | Export endpoint exists but response format is undocumented; no frontend trigger |
| 35 | Onboarding | No onboarding flow — new users land on a blank dashboard with no guidance |
| 36 | Frontend | Session list hardcoded `limit=50`, no "load more" / pagination control |
| 39 | Auth | Invitation emails not sent when org owner adds a member |
| 42 | Frontend | No 429 (rate limit) handling in UI — user sees a generic error with no retry guidance |
| 43 | Frontend | react-force-graph-3d requires WebGL — no fallback for browsers without WebGL (older devices, some Linux Firefox configs) |
| 44 | Frontend | 3D graph (Three.js) not lazy-loaded — adds ~300KB JS to initial page load for all users |
| 45 | Frontend | No graph export (PNG / SVG / JSON download from the DAG explorer) |
| 46 | Frontend | No read-only session share link |
| 47 | Frontend | No diff view showing fact state changes over time |
| 48 | Frontend | Theme stored in localStorage only — flickers on hard refresh (no server-side cookie for theme) |

---

## Tier 4 — Hardening (before Series A / scale)

| # | Area | Item |
|---|------|------|
| 52 | Observability | No frontend error tracking (Sentry or equivalent) — browser crashes are invisible |
| 53 | Observability | No product analytics — no way to know which features beta users actually use |
| 54 | Testing | No handler-level integration tests against a real Postgres instance |
| 55 | Testing | No `vlx` CLI behavioral tests |
| 56 | Testing | No frontend component tests (Vitest / React Testing Library) |
| 57 | Docs | OpenAPI spec not auto-generated from handlers — drifts silently |
| 60 | Frontend | No mobile/responsive pass on dashboard or session viewer |
| 61 | Frontend | No OG/meta tags — shared links show no preview |
| 62 | Backend | `vlx` CLI commands documented but no automated test verifies CLI output format doesn't regress |
| 63 | Backend | GLiNER benchmark comparison pending — needs a machine with ≥1.5 GB free RAM; add run to BENCHMARK_HISTORY.md when available |

---

## Fixed in v0.7.0 hardening pass (17 items)

| # | Item | Where |
|---|------|-------|
| 4 | Graceful shutdown — SIGTERM drains in-flight requests (30s timeout) | `main.go` |
| 6 | SSRF — webhook URL validated against private/loopback/link-local ranges before dispatch | `api/verification.go` |
| 9 | Engine cold-start note: revised — not a data-loss risk; data rebuilt from Postgres journal (see note in audit) | — |
| 10 | Max session count enforced per plan (`free=50`, `pro=500`, `enterprise=∞`) | `api/console_contracts.go` |
| 21 | Bootstrap key already off in prod (`isDevLikeEnv()=false`) — confirmed, no change needed | — |
| 22 | Engine cache already has LRU eviction (`PerformEvictionSweep`) — confirmed, no change needed | — |
| 24 | Postgres pool config: `MaxConns=20`, `MinConns=2`, `MaxConnLifetime=30m`, `MaxConnIdleTime=5m` | `store/postgres/store.go` |
| 25 | SliceCache TTL cleanup goroutine — evicts entries older than 15 min every 10 min | `api/server.go` |
| 26 | Expiry sweep jitter — ±10% random jitter prevents thundering herd | `api/server.go` |
| 37 | Global facts list — paginated (`limit`/`offset`, default limit 100, max 1000) | `api/global_facts.go` |
| 38 | Plan enforcement — session count limit enforced at create time per subscription tier | `api/enterprise_controls.go`, `api/console_contracts.go` |
| 40 | Redis fallback warning — logs `Warn` at startup and sets `RedisUnavailable` flag on server | `main.go` |
| 41 | Idempotency Redis TTL already set (24h `SET … EX`) — confirmed, no change needed | — |
| 49 | Journal purge — `POST /v1/admin/purge-journal` + weekly scheduled ticker; safe because replays from snapshots | `store/postgres/store.go`, `api/server.go` |
| 50 | Feature flag startup log — structured log at startup lists bootstrap key, Redis, GLiNER, backend, lite mode | `main.go` |
| 51 | Trace ID middleware — all requests get a trace ID before auth; propagated to response header and context | `api/server.go` |
| 58 | Log level env var — `VELARIX_LOG_LEVEL=debug\|info\|warn\|error` | `main.go` |
| 59 | Circuit breaker on SRL service — opens after 5 consecutive 5xx/network failures; half-open probe after 30s | `extractor/srl_pipeline.go` |

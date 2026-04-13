# Changelog

All notable changes to Velarix will be documented in this file.

## [Unreleased]

### Added
- **Extractor**: Introduced the `extractor` package to convert raw LLM text into atomic factual assertions. Supports compound claim decomposition and dependency tracking.
- **Extractor**: Added automatic `assertion_kind` classification (empirical|uncertain|hypothetical|fictional) and mapping into `core.Fact`.
- **Global Truth**: Wired up `GlobalTruth` with org-wide global facts endpoints (`/v1/global/facts`) and session subscriptions.
- **Operations**: Added an expiry sweep ticker to persist `fact_expired` events and invalidate downstream dependents promptly (`VELARIX_EXPIRY_SWEEP_INTERVAL_SECONDS`).
- **Slices**: Added freshness-aware slice ranking via `asserted_at` plus exponential decay (`VELARIX_SLICE_FRESHNESS_DECAY_HOURS`, `VELARIX_SLICE_FRESHNESS_WEIGHT`).
- **Metrics**: Added `velarix_facts_expired_total` and `velarix_global_fanout_total`.
- **Verification**: Added fact verification metadata (`requires_verification`, `verification_status`, `verified_at`, etc), persisted as `fact_verification` journal events for replay.
- **Verification**: Added admin fact verification endpoint `POST /v1/s/{session_id}/facts/{fact_id}/verify`.
- **Verification**: Added optional verification webhook automation (`VELARIX_VERIFICATION_WEBHOOK_URL`, `VELARIX_VERIFICATION_WEBHOOK_TIMEOUT_SECONDS`).
- **Governance**: Added grounding/verification policy controls to prevent fabricated or stale premises from grounding execution-critical facts and decisions.
- **Consistency**: Added optional auto-flagging of facts for review when contradictions are detected (`VELARIX_AUTO_FLAG_REVIEW_ON_CONTRADICTION`).
- **Benchmark Harness**: Added a reproducible, standalone benchmark harness (`benchmark/harness.go` and `tests/reproducibility/hallucination_benchmark.py`) for long-horizon contradiction evaluation.
- **SDK Additions**: Added integrations for CrewAI, LlamaIndex, and LangGraph to the Python SDK (`sdks/python/velarix/integrations`). 
- **SDK Additions**: Added `client.global_facts.*` helpers to the Python SDK for global facts.
- **Repository Hygiene**: Standardized documentation across README, CONTRIBUTING, and DEPLOYMENT guides.

### Fixed
- **Core Engine**: Fixed a TOCTOU (Time-Of-Check to Time-Of-Use) race condition in `GetStatus()` to eliminate the window between read/write locks.
- **Core Engine**: Prevented hypothetical/fictional facts from grounding empirical/uncertain derived conclusions.
- **Core Engine**: Added `asserted_at` timestamps to facts at assertion time for freshness scoring.
- **API**: Session reload and revalidation now re-apply `fact_expired` events so expired premises invalidate descendants on reload.
- **Core Engine**: Fixed negated parent dependencies in `ExplainReasoning()` to properly strip the `!` prefix and route them to `NegatedParents`.
- **Extractor**: Fixed topological sorting logic in `Extract()` to ensure root facts are processed before derived facts.
- **Benchmark Harness**: Fixed the `splitSentences` logic in `reconstructGroundedResponse` to preserve punctuation.

### Security
- **Auth**: Enforced member-only registration by removing open/public registration flows and introducing `VELARIX_ENABLE_PUBLIC_REGISTRATION`.
- **Auth**: Implemented invitation account takeover prevention by ensuring a user does not already exist when accepting an invitation.
- **Auth**: Removed the insecure hardcoded JWT fallback secret (`velarix_dev_insecure_jwt_secret_change_me`).
- **API**: Enforced request body size limits (2MB) on the `extract-and-assert` endpoint to prevent DoS attacks.
- **Operations**: Restricted global backup (`/v1/org/backup`) and restore (`/v1/org/restore`) endpoints strictly to the platform admin (`orgID == "admin"`).

# Changelog

All notable changes to Velarix will be documented in this file.

## [Unreleased]

### Added
- **Extractor**: Introduced the `extractor` package to convert raw LLM text into atomic factual assertions. Supports compound claim decomposition and dependency tracking.
- **Benchmark Harness**: Added a reproducible, standalone benchmark harness (`benchmark/harness.go` and `tests/reproducibility/hallucination_benchmark.py`) for long-horizon contradiction evaluation.
- **SDK Additions**: Added integrations for CrewAI, LlamaIndex, and LangGraph to the Python SDK (`sdks/python/velarix/integrations`). 
- **Repository Hygiene**: Standardized documentation across README, CONTRIBUTING, and DEPLOYMENT guides.

### Fixed
- **Core Engine**: Fixed a TOCTOU (Time-Of-Check to Time-Of-Use) race condition in `GetStatus()` to eliminate the window between read/write locks.
- **Core Engine**: Fixed negated parent dependencies in `ExplainReasoning()` to properly strip the `!` prefix and route them to `NegatedParents`.
- **Extractor**: Fixed topological sorting logic in `Extract()` to ensure root facts are processed before derived facts.
- **Benchmark Harness**: Fixed the `splitSentences` logic in `reconstructGroundedResponse` to preserve punctuation.

### Security
- **Auth**: Enforced member-only registration by removing open/public registration flows and introducing `VELARIX_ENABLE_PUBLIC_REGISTRATION`.
- **Auth**: Implemented invitation account takeover prevention by ensuring a user does not already exist when accepting an invitation.
- **Auth**: Removed the insecure hardcoded JWT fallback secret (`velarix_dev_insecure_jwt_secret_change_me`).
- **API**: Enforced request body size limits (2MB) on the `extract-and-assert` endpoint to prevent DoS attacks.
- **Operations**: Restricted global backup (`/v1/org/backup`) and restore (`/v1/org/restore`) endpoints strictly to the platform admin (`orgID == "admin"`).

# Velarix: Final Production Implementation Tracker

## 1. Core Engine & Epistemic Reliability
...
## 2. Storage & Persistence (BadgerDB v4 Strategy)
...
## 3. The Extraction & Interceptor Layer
...
## 4. SaaS Infrastructure (Un-mocking the Identity)
- [ ] **Real Identity Provider**:
    - [ ] **Argon2 Hashing**: Implement secure password storage with `golang.org/x/crypto/argon2`.
    - [ ] **Full Auth Loop**: Implement `POST /auth/register`, `/auth/login`, `/auth/reset-request` (logs to console), and `/auth/reset-confirm`.
    - [ ] **Multi-Key Management**: Support multiple labeled API keys per user with last-used tracking.
    - [ ] **Revocation & Security**: Ensure revoked keys return the same 401 as non-existent keys.
- [ ] **Organization Multi-tenancy**:
    - Implement `Organization` nodes (Users belong to Orgs, Sessions belong to Orgs).
...

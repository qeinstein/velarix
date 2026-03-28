# Velarix Security & Compliance

Velarix is engineered for the highest levels of security, particularly for healthcare and other regulated industries. Every component, from the API to the persistence layer, is designed with security as a first-class citizen.

## 🔒 Security Principles

- **Zero Trust Architecture**: Every request is authenticated and authorized against a strict organization-based permission model.
- **Fail Closed**: In the event of a security or database error, Velarix defaults to a "failed" state rather than allowing unauthorized access.
- **Auditability by Default**: Every state-modifying action is permanently recorded in a SHA-256 verified journal.

## 🛡️ Authentication & Authorization

Velarix utilizes a two-tier authentication system:

1.  **JWT-Based (User)**: For console access, uses Argon2id password hashing and JWT tokens for secure session management. Production requires `VELARIX_JWT_SECRET`.
2.  **API Key-Based (Service)**: For SDK and automated interactions. API keys are **shown once** at creation/rotation and are **not persisted in plaintext** (stored as SHA-256 hashes + redacted prefixes for UI).

### Tenant Isolation

Velarix enforces strict **Organization-level (OrgID)** isolation. Every session is bound to an organization, and every API handler verifies that the requester belongs to that organization before processing any data. This prevents cross-tenant data leaks.

### Least Privilege (Scopes + Roles)

- **API key scopes**: `read`, `write`, `export`, `admin`
- **User roles**: `admin`, `member`, `auditor` (read/export only)

The API checks scope/role on every request and restricts sensitive surfaces like exports and access logs.

## 🔏 Data Encryption

- **In-Transit**: Velarix is designed to run behind an SSL/TLS-terminated reverse proxy (e.g., Nginx, Envoy, or cloud-native load balancers).
- **At-Rest**: Velarix requires a 32-byte encryption key (`VELARIX_ENCRYPTION_KEY`) in production. All data stored in BadgerDB is encrypted using **AES-256** in CTR mode.

## 📜 Compliance & Audit Trails

Velarix provides the foundation for SOC2 and HIPAA compliance by ensuring that every decision made by an AI agent is deterministic and auditable.

### SHA-256 Verified Journal

Every session maintains a chronological journal of assertions and invalidations.
- **Actor Attribution**: Every entry includes the `actor_id` (User Email or API Key Label) and a millisecond-resolution timestamp.
- **Integrity Verification**: Audit exports (CSV/PDF) include a SHA-256 hash of the entire history, allowing organizations to verify that the audit trail has not been tampered with.

### Access Logs

Velarix records per-request access logs (who accessed what and when) and exposes them via `GET /v1/org/access-logs` (restricted to auditor/admin via `export` scope). Retention is configurable via `PATCH /v1/org/settings`.

### HIPAA/GDPR “Right to be Forgotten” (clarification)

The `invalidate` and `slice` mechanics support **truth maintenance** and retraction (removing facts from the agent’s valid context). They are not a substitute for legal deletion requirements by themselves. For regulated deletion, implement a data deletion workflow that removes the underlying stored payloads and ensures exports/retention align with policy.

---
*Velarix: Secure-by-design for high-stakes AI.*

# Velarix Security & Compliance

Velarix is engineered for the highest levels of security, particularly for healthcare and other regulated industries. Every component, from the API to the persistence layer, is designed with security as a first-class citizen.

## 🔒 Security Principles

- **Zero Trust Architecture**: Every request is authenticated and authorized against a strict organization-based permission model.
- **Fail Closed**: In the event of a security or database error, Velarix defaults to a "failed" state rather than allowing unauthorized access.
- **Auditability by Default**: Every state-modifying action is permanently recorded in a SHA-256 verified journal.

## 🛡️ Authentication & Authorization

Velarix utilizes a two-tier authentication system:

1.  **JWT-Based (User)**: For console access, uses Argon2id password hashing and JWT tokens for secure session management.
2.  **API Key-Based (Service)**: For SDK and automated interactions. API keys can be labeled, rotated, and revoked instantly via the API or console.

### Tenant Isolation

Velarix enforces strict **Organization-level (OrgID)** isolation. Every session is bound to an organization, and every API handler verifies that the requester belongs to that organization before processing any data. This prevents cross-tenant data leaks.

## 🔏 Data Encryption

- **In-Transit**: Velarix is designed to run behind an SSL/TLS-terminated reverse proxy (e.g., Nginx, Envoy, or cloud-native load balancers).
- **At-Rest**: Velarix requires a 32-byte encryption key (`VELARIX_ENCRYPTION_KEY`) in production. All data stored in BadgerDB is encrypted using **AES-256** in CTR mode.

## 📜 Compliance & Audit Trails

Velarix provides the foundation for SOC2 and HIPAA compliance by ensuring that every decision made by an AI agent is deterministic and auditable.

### SHA-256 Verified Journal

Every session maintains a chronological journal of assertions and invalidations.
- **Actor Attribution**: Every entry includes the `actor_id` (User Email or API Key Label) and a millisecond-resolution timestamp.
- **Integrity Verification**: Audit exports (CSV/PDF) include a SHA-256 hash of the entire history, allowing organizations to verify that the audit trail has not been tampered with.

### HIPAA/GDPR "Right to be Forgotten"

The `invalidate` and `slice` mechanics provide a powerful way to handle data retraction. When a patient revokes consent or requests data deletion, invalidating the root fact ensures that every dependent belief is instantly and deterministically removed from the agent's context.

---
*Velarix: Secure-by-design for high-stakes AI.*

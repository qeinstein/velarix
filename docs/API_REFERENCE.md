# Velarix API Reference (v1)

Velarix provides a versioned REST API for managing reasoning sessions, asserting facts, and performing epistemic analysis. All session-scoped endpoints are prefixed with `/v1/s/{session_id}`.

## 🔑 Authentication

Velarix supports two types of authentication:
1.  **JWT**: Primarily for the Velarix Console. Obtain a token via `/v1/auth/login`.
2.  **API Key**: For SDKs and automated integrations. Use the `Authorization: Bearer <your_vx_key>` header.

---

## 🔐 Auth & Key Management

### `POST /v1/auth/login`
Authenticate a user and receive a JWT.
- **Body**: `{ "email": "...", "password": "..." }`
- **Returns**: `{ "token": "..." }`

### `POST /v1/auth/register`
Create a new user account.
- **Body**: `{ "email": "...", "password": "..." }`

### `GET /v1/keys`
List all API keys associated with the authenticated user.

### `POST /v1/keys/generate`
Generate a new labeled API key.
- **Body**: `{ "label": "Production" }`

### `DELETE /v1/keys/{key}`
Revoke an API key.

---

## 🧩 Sessions & Facts

### `GET /v1/sessions`
List all active sessions currently held in memory.

### `POST /v1/s/{session_id}/facts`
Assert a new fact to the reasoning graph.
- **Body (Root Fact)**:
  ```json
  {
    "id": "hipaa_consent",
    "is_root": true,
    "manual_status": 1.0,
    "payload": { "signed_at": "2026-03-20" }
  }
  ```
- **Body (Derived Fact)**:
  ```json
  {
    "id": "phi_processing",
    "justification_sets": [["hipaa_consent"]],
    "payload": { "scope": "read_only" }
  }
  ```

### `GET /v1/s/{session_id}/facts`
List all facts in a session.
- **Query Params**: `valid=true` (optional, return only logically valid facts).

### `GET /v1/s/{session_id}/facts/{id}`
Retrieve a single fact by its ID.

### `POST /v1/s/{session_id}/facts/{id}/invalidate`
Manually invalidate a root fact, triggering causal collapse.

### `GET /v1/s/{session_id}/slice`
Retrieve the logically valid subset of facts, formatted for LLM consumption.
- **Query Params**:
  - `format`: `json` or `markdown`.
  - `max_facts`: Limit the number of facts returned.

---

## 📈 Analysis & Observability

### `GET /v1/s/{session_id}/facts/{id}/why`
Retrieve the justification tree (explanation) for a specific fact.

### `GET /v1/s/{session_id}/facts/{id}/impact`
Calculate the "blast radius" (downstream effects) of retracting a specific fact.

### `POST /v1/s/{session_id}/revalidate`
Clear the kernel state and replay the session's full journal through all current validation rules.

### `GET /v1/s/{session_id}/events`
Open a Server-Sent Events (SSE) stream for real-time belief updates.

---

## 📜 History & Audit

### `GET /v1/s/{session_id}/history`
Retrieve the full chronological journal (audit trail) for a session.

### `GET /v1/s/{session_id}/export`
Generate a SOC2-ready audit report.
- **Query Params**: `format` (`csv` or `pdf`).

---

## ⚙️ System & Metrics

### `GET /health`
Basic health status, version, and uptime.

### `GET /v1/org/usage`
Retrieve atomic usage metrics for your organization (facts asserted, API requests, etc.).

---
*Velarix: Building the trust layer for autonomous healthcare.*

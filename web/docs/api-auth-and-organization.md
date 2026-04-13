---
title: "API: Auth And Organization"
description: "Reference the authentication, API key, session administration, org settings, account, and console-style endpoints exposed outside the core session API."
section: "API Reference"
sectionOrder: 3
order: 4
---

# Auth and org endpoints

These routes are only available in full mode. Lite mode does not register them.

## Authentication overview

Velarix accepts two auth models:

- JWT-backed user sessions, usually set by the login flow
- bearer API keys, looked up by SHA-256 hash and scoped to `read`, `write`, `export`, and `admin`

The auth middleware also has a bootstrap path: if `VELARIX_API_KEY` is configured and bootstrap admin mode is still enabled, that key is accepted as an admin credential.

## `POST /v1/auth/register`

Create a user account. The same handler is also exposed at `/auth/register`.

Request body:

- `email` (`string`, required)
- `password` (`string`, required)

Response body:

- registration status payload from the auth handler

Notes:

- the server uses Argon2id for password hashing
- organization assignment is handled by the registration flow or invitation acceptance, not by a client-supplied org ID

## `POST /v1/auth/login`

Authenticate a user and establish a JWT-backed session. Also exposed at `/auth/login`.

Request body:

- `email` (`string`, required)
- `password` (`string`, required)

Response body:

- login status payload
- auth cookies may be set depending on deployment

## `POST /v1/auth/logout`

Clear the current auth session. Also exposed at `/auth/logout`.

## `POST /v1/auth/reset-request`

Start password reset.

Request body:

- `email` (`string`, required)

If SMTP is configured, the server sends an email containing a reset link or token. If `VELARIX_BASE_URL` is set, the reset link is built as `${VELARIX_BASE_URL}/reset-password?...`.

## `POST /v1/auth/reset-confirm`

Complete password reset.

Request body:

- `email` (`string`, required)
- `token` (`string`, required)
- `new_password` (`string`, required)

## API key endpoints

### `POST /v1/keys/generate`

Issue a new API key for a user.

Request body:

- `email` (`string`, required)
- `label` (`string`, required)
- `scopes` (`string[]`, optional)

Response body:

- `APIKeyView`

`APIKeyView` contains:

- `id`
- `key`
- `key_prefix`
- `key_last4`
- `label`
- `created_at`
- `last_used_at`
- `expires_at`
- `is_revoked`
- `scopes`

Important behavior:

- the raw key is only returned on create and rotate responses
- stored records keep hash and redacted display fields, not the raw token

### `GET /v1/keys`

List the caller's visible API keys.

### `DELETE /v1/keys/{key}`

Revoke a key.

### `POST /v1/keys/{key}/rotate`

Rotate a key and return a fresh issued key payload.

## Account endpoints

### `GET /v1/me`

Return the authenticated principal.

Response body:

- `email`
- `org_id`
- `role`

### `POST /v1/me/change-password`

Request body:

- `current_password`
- `new_password`

Behavior:

- `new_password` must be at least 8 characters
- current password must match the stored Argon2 hash

### `GET /v1/me/onboarding`

Return the caller's onboarding state map.

### `POST /v1/me/onboarding`

Patch onboarding flags.

Request body:

- arbitrary `map[string]bool`

Response body:

- the updated onboarding map

## Organization endpoints

### `GET /v1/org`

Return the current organization.

Response body:

- `id`
- `name`
- `created_at`
- `is_suspended`
- `settings`

### `PATCH /v1/org`

Admin only.

Request body:

- `name` (`string`, optional)
- `is_suspended` (`boolean`, optional)

### `GET /v1/org/settings`

Return org settings, including defaults when values are absent:

- `retention_days_activity` default `30`
- `retention_days_access_logs` default `30`
- `retention_days_notifications` default `30`
- `rate_limit_rpm` default `60`
- `rate_limit_window_seconds` default `60`

### `PATCH /v1/org/settings`

Admin only. Allowed keys:

- `retention_days_activity`
- `retention_days_access_logs`
- `retention_days_notifications`
- `rate_limit_rpm`
- `rate_limit_window_seconds`

Validation:

- retention values must be integers in `1..3650`
- rate limit values must be integers in `1..100000`

### `GET /v1/org/usage`

Return aggregate org metrics.

### `GET /v1/org/usage/timeseries`

Return minute-bucket metric series.

### `GET /v1/org/usage/breakdown`

Return grouped usage breakdowns.

### `GET /v1/org/sessions`

List org sessions.

Query parameters:

- `cursor`
- `limit`

Response body:

- `items` (`OrgSessionMeta[]`)
- `next_cursor`

Each session item contains:

- `id`
- `name`
- `description`
- `created_at`
- `last_activity_at`
- `fact_count`
- `archived`

### `POST /v1/org/sessions`

Create an empty session record.

Request body:

- `name` (`string`, optional)
- `description` (`string`, optional)

Response body:

- `id`
- `name`
- `description`

IDs are generated as `s_<hex>`.

### `PATCH /v1/org/sessions/{id}`

Patch session metadata.

Request body:

- `name` (`string`, optional)
- `description` (`string`, optional)

### `DELETE /v1/org/sessions/{id}`

Archive a session and unload it from the in-memory engine cache.

Response body:

- `status`
- `id`

### `GET /v1/s/{session_id}/summary`

Return a small session summary for dashboards.

Response body:

- `id`
- `fact_count`
- `enforcement_mode`
- `schema_set`
- `status`

## Search, audit, and compliance endpoints

### `GET /v1/org/search`

Search org documents.

Query parameters:

- `q`
- `type`
- `status`
- `subject`
- `cursor`
- `limit` (max `200`, default `50`)

Response body:

- `items`
- `next_cursor`

### `GET /v1/org/activity`

List org journal activity with retention enforcement.

### `GET /v1/org/access-logs`

List org access logs with retention enforcement.

### `GET /v1/org/compliance-export`

Export org-level data.

Common query parameters:

- `format`: `json` or `ndjson`
- `limit`

## Notification, integration, and user management

### `GET /v1/org/notifications`

Return notification pages as:

- `items`
- `next_cursor`

### `POST /v1/org/notifications/{id}/read`

Mark one notification as read.

### `GET /v1/org/integrations`

List integrations.

### `POST /v1/org/integrations`

Admin only.

Request body:

- `name`
- `kind`
- `enabled`
- `config`

### `PATCH /v1/org/integrations/{id}`

Admin only.

Request body:

- `name`
- `enabled`
- `config`

### `DELETE /v1/org/integrations/{id}`

Admin only.

### `GET /v1/org/users`

Admin only. Return:

- `{ "items": ["user1@example.com", ...] }`

## Invitations

### `GET /v1/org/invitations`

List org invitations.

### `POST /v1/org/invitations`

Admin only.

Request body:

- `email`
- `role`

Allowed roles:

- `admin`
- `member`
- `auditor`

Response body:

- `status`
- `invitation`
- `token`

### `POST /v1/org/invitations/{id}/revoke`

Admin only. Revoke an invitation.

### `POST /v1/invitations/accept`

Accept an invitation.

Request body:

- `token`
- `password`

Important behavior:

- if the invited email already belongs to an existing account, unauthenticated acceptance is rejected
- in that case the user must authenticate as the existing account before joining the new org

## Billing, support, policy, and docs endpoints

### `GET /v1/billing/subscription`

Return the current subscription. When no billing record exists, the handler returns a synthetic default object with plan `free` and status `active`.

### `PATCH /v1/billing/subscription`

Admin only.

Request body:

- `plan`
- `status`
- `billing_email`

### `GET /v1/support/tickets`

Return `{ "items": [...] }`.

### `POST /v1/support/tickets`

Request body:

- `subject`
- `body`

Creates a ticket with status `open`.

### `PATCH /v1/support/tickets/{id}`

Request body:

- `status`: `open` or `closed`

### `GET /v1/policies`

Return `{ "items": [...] }`.

### `POST /v1/policies`

Admin only.

Request body:

- `name`
- `enabled`
- `rules`

### `PATCH /v1/policies/{id}`

Admin only.

Request body:

- `name`
- `enabled`
- `rules`

### `DELETE /v1/policies/{id}`

Admin only.

### `GET /v1/legal/terms`

Return the built-in terms document.

### `GET /v1/legal/privacy`

Return the built-in privacy document.

### `GET /v1/docs/pages`

List markdown pages available from the server-side `markdown/` directory.

### `GET /v1/docs/pages/{slug}`

Return:

- `slug`
- `content`

## Example: create a session through the org API

```bash
curl -sS -X POST http://localhost:8080/v1/org/sessions \
  -H "Authorization: Bearer $VELARIX_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "name":"Invoice review",
    "description":"Payment decision for inv-1042"
  }'
```

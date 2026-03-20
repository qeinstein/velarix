# Velarix Error Codes & Troubleshooting

Velarix uses standard HTTP status codes and detailed JSON error messages to communicate issues.

## 🛑 Common Error Codes

### 401 Unauthorized
- **Cause**: Invalid or expired API key or JWT token.
- **Resolution**: Verify your `Authorization` header and ensure your API key has not been revoked.

### 403 Forbidden
- **Cause**: Attempting to access a session or resource belonging to a different organization (Tenant Isolation).
- **Resolution**: Ensure you are using the correct OrgID/API Key for the session.

### 404 Not Found
- **Cause**: The requested fact, session, or key does not exist.
- **Resolution**: Check the ID of the resource you are requesting.

### 429 Too Many Requests
- **Cause**: You have exceeded the rate limit of 60 requests per minute.
- **Resolution**: Implement exponential backoff in your application or request a quota increase.

### 400 Bad Request
- **Cause**: Invalid JSON, schema validation failure, or logical violation (e.g., cycle detected in the graph).
- **Resolution**:
    - **Schema Failure**: Verify your fact `payload` matches the session's JSON schema.
    - **Cycle Violation**: Ensure your fact justifications do not create a circular dependency.

## 🛠 Troubleshooting Scenarios

### Fact is Not Appearing in `get_slice()`
- **Cause**: The fact has a resolved status below the `ConfidenceThreshold` (0.5), or one of its required parents has been invalidated.
- **Check**: Use `GET /v1/s/{session_id}/facts/{id}/why` to see the justification tree and identify why the fact is currently considered invalid.

### Session Startup is Slow
- **Cause**: Large number of journal entries to replay since the last snapshot.
- **Resolution**: Velarix snapshots automatically every 50 mutations or every 5 minutes. If startup is consistently slow, check the health of your disk I/O.

### "Decryption Failed" on Startup
- **Cause**: Incorrect `VELARIX_ENCRYPTION_KEY` provided.
- **Resolution**: Ensure the 32-byte encryption key matches the one used when the data was originally written. Data cannot be recovered without the original key.

### Metrics are Missing in Prometheus
- **Cause**: The `/metrics` endpoint is not being scraped.
- **Check**: Ensure your Prometheus configuration includes the Velarix server as a target and that the `/metrics` path is accessible.

---
*Velarix: Transparent and deterministic error handling.*

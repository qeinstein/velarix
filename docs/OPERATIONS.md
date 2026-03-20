# Velarix Operations & Maintenance

This guide covers the day-to-day operations, monitoring, and maintenance of a Velarix cluster.

## 📊 Monitoring & Observability

Velarix exports high-resolution metrics for monitoring via a Prometheus-compatible `/metrics` endpoint.

### Key Metrics to Monitor

- `velarix_api_requests_total`: Total count of API requests by path and status code.
- `velarix_active_sessions`: Number of sessions currently loaded in RAM.
- `velarix_fact_assertion_latency_ms`: Histogram of fact assertion latency.
- `velarix_prune_latency_ms`: Histogram of invalidation/pruning latency.
- `velarix_slo_success_rate`: Tracking success vs failure for SLO monitoring (target 99.9%).

### System Health

Velarix provides two health check endpoints:
- `GET /health`: Basic connectivity and uptime check.
- `GET /health/full`: Detailed system status, including disk usage and memory allocation (requires admin role).

## 💾 Backups & Recovery

Velarix uses **BadgerDB v4** as its underlying storage engine. We recommend a multi-tier backup strategy.

### Automated Backups

Velarix includes an automated daily backup ticker that saves a binary backup to `velarix_backup_<timestamp>.bak`. This can be customized in the server configuration.

### Manual Backups

For manual or ad-hoc backups, use the admin API:
```bash
# Export a full backup of the system state
curl -X GET http://localhost:8080/v1/org/backup -H "Authorization: Bearer <admin_key>" --output backup.bak
```

### Restoration

To restore a system from a backup:
```bash
curl -X POST http://localhost:8080/v1/org/restore -H "Authorization: Bearer <admin_key>" --data-binary @backup.bak
```
**Note**: Restoration is a destructive operation and will overwrite any current data in the database.

## ⚖️ Rate Limiting

Velarix implements a persistent rate-limiting system to ensure stability and prevent abuse.
- **Default Limit**: 60 requests per minute (RPM) per API key.
- **Persistence**: Quotas are stored in BadgerDB and persist across server restarts.

When a limit is exceeded, the API will return a `429 Too Many Requests` status code.

## 🧹 Session Eviction

To manage memory usage, Velarix employs an automated **Eviction Sweep** every 5 minutes:
- **Base Eviction**: Any session that hasn't been accessed in 30 minutes is evicted from RAM.
- **Aggressive Eviction**: If heap memory allocation exceeds 1GB, the oldest sessions (LRU) are evicted until memory usage stabilizes.

Evicted sessions are still stored on disk and will be automatically reloaded on the next access via the **Hybrid Boot** strategy.

---
*Velarix: Engineered for production reliability.*

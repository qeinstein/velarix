# Velarix: Benchmarking And Deployment

Velarix ships with a deterministic contradiction benchmark and a production deployment path centered on shared infrastructure.

---

## Part 1: Benchmarking Velarix

The benchmark exercises long-horizon contradiction handling.

The benchmark compares four strategies:

- `tms`
- `plain_rag`
- `self_reflection`
- `memory_refresh`

### 1. Run The Benchmark

From the project root:

```bash
python3 tests/reproducibility/hallucination_benchmark.py --spawn-server --steps 120
```

Optional flags:

```bash
python3 tests/reproducibility/hallucination_benchmark.py \
  --spawn-server \
  --steps 240 \
  --contradiction-interval 17 \
  --output benchmark-results.json
```

If a Velarix server is already running:

```bash
python3 tests/reproducibility/hallucination_benchmark.py \
  --base-url http://127.0.0.1:8080 \
  --api-key your_api_key
```

### 2. Benchmark Metrics

The output includes:

- `task_success_rate`
- `consistency_rate`
- `stale_action_rate`
- `missing_context_rate`
- `context_recall_rate`
- `retraction_efficiency`
- `max_latent_stale_plans`
- `runtime_s`

### 3. Workload Shape

Each run executes a long mission across research, coding, and tool-use topics.

At fixed intervals it injects a contradiction by invalidating the previously trusted version of a topic and replacing it with a new one.

The `tms` strategy routes those updates through Velarix facts, invalidation, decisions, and query-aware slices.

The baseline strategies simulate:

- plain retrieval of older context
- self-reflection with newest remembered items
- simple memory refresh windows

### 4. Result Interpretation

The benchmark is designed to show:

- how often stale actions survive a contradiction
- how often the correct plan remains present in context
- how efficiently retraction removes outdated plans

It is a workload benchmark for contradiction handling, state correction, and execution safety.

---

## Part 2: Deploying Velarix

Velarix is deployed as a stateless API tier backed by shared infrastructure.

### Recommended Production Shape

- compute: containerized Go API
- primary store: Postgres
- coordination: Redis
- browser app: separate frontend deployment pointed at the API origin

### Production Requirements

Set these environment variables:

- `VELARIX_ENV=prod`
- `VELARIX_STORE_BACKEND=postgres`
- `VELARIX_POSTGRES_DSN=...`
- `VELARIX_JWT_SECRET=...`
- `VELARIX_ALLOWED_ORIGINS=...`

Recommended for multi-instance deployments:

- `VELARIX_REDIS_URL=...`

Only set these if you intentionally need them:

- `VELARIX_ENABLE_BOOTSTRAP_ADMIN_KEY=true`
- `VELARIX_API_KEY=...`
- `VELARIX_ALLOW_BADGER_PROD=true`

### Google Cloud Platform Reference Shape

For GCP deployments, the recommended stack is:

1. Cloud Run for the Go API
2. Cloud SQL for Postgres
3. Memorystore for Redis
4. a separate frontend deployment for the Next.js app

### Example Cloud Run Deployment

```bash
gcloud run deploy velarix-api \
  --image gcr.io/$PROJECT_ID/velarix-engine:latest \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --set-env-vars="VELARIX_ENV=prod" \
  --set-env-vars="VELARIX_STORE_BACKEND=postgres" \
  --set-env-vars="VELARIX_POSTGRES_DSN=postgres://postgres:YOUR_DB_PASSWORD@YOUR_CLOUD_SQL_IP:5432/velarix?sslmode=disable" \
  --set-env-vars="VELARIX_REDIS_URL=redis://YOUR_MEMORYSTORE_IP:6379" \
  --set-env-vars="VELARIX_JWT_SECRET=YOUR_SECURE_JWT_SECRET" \
  --set-env-vars="VELARIX_ALLOWED_ORIGINS=https://app.yourdomain.com" \
  --vpc-connector=YOUR_VPC_CONNECTOR_NAME
```

### Browser App Deployment

Set the frontend environment variable:

```bash
NEXT_PUBLIC_VELARIX_API_URL=https://api.yourdomain.com
```

The web console uses cookie-based auth against that API origin.

### Operating Rule

Run Postgres as the source of truth.

Treat Redis as required whenever more than one API instance is handling live traffic.


## THis is a scalfold for now, not verified benchmarks
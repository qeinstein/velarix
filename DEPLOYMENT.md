# Velarix Deployment Guide

Complete, step-by-step guide for deploying Velarix to Google Cloud Run at `velarix.dev`. Every command references the actual environment variable names, file paths, port numbers, and configuration values in this repository. Nothing is invented.

---

## Table of Contents

1. [Namespace and SDK Publication](#1-namespace-and-sdk-publication)
2. [Prerequisites](#2-prerequisites)
3. [Containerisation](#3-containerisation)
4. [Cloud SQL (PostgreSQL)](#4-cloud-sql-postgresql)
5. [Memorystore (Redis)](#5-memorystore-redis)
6. [Secret Manager](#6-secret-manager)
7. [Cloud Run Deployment](#7-cloud-run-deployment)
8. [Custom Domain](#8-custom-domain)
9. [Web Frontend](#9-web-frontend)
10. [CI/CD](#10-cicd)
11. [Production Checklist](#11-production-checklist)

---

## Correct Order of Operations

The sequence matters. Namespaces must be claimed before anything is published or pushed, because names are first-come-first-served and cannot be reclaimed once taken by another party.

```
1. Claim velarix org on GitHub
2. Claim velarix namespace on PyPI
3. Claim velarix namespace on Docker Hub
4. Prepare and publish the Python SDK to PyPI
5. Push the codebase to GitHub (github.com/velarix/velarix)
6. Provision Google Cloud infrastructure (SQL, Redis, Secrets)
7. Build and push Docker image to Artifact Registry
8. Deploy backend to Cloud Run
9. Deploy frontend to Cloud Run
10. Point velarix.dev DNS to Cloud Run
11. Open source release (tag v0.1.0, publish GitHub release)
```

---

## 1. Namespace and SDK Publication

### 1.1 Claim the `velarix` Organisation on GitHub

1. Log in to GitHub as the account that will own the organisation.
2. Go to [https://github.com/organizations/plan](https://github.com/organizations/plan) and select the free tier.
3. Set the organisation name to `velarix`.
4. Add team members with Owner or Member roles as needed.

Once the org exists, create the main repository:

```bash
# Install the GitHub CLI if not already present
# https://cli.github.com

gh auth login

# Create the repository under the org
gh repo create velarix/velarix \
  --public \
  --description "Velarix — open-source belief graph and reasoning engine" \
  --homepage "https://velarix.dev"
```

Add the remote and push the codebase:

```bash
git remote add origin git@github.com:velarix/velarix.git
git push -u origin main
git push origin --tags
```

### 1.2 Claim the `velarix` Namespace on PyPI

PyPI namespaces are claimed by registering a project. The fastest way to reserve the name before publishing a full release is to upload a minimal package immediately.

**First, create a PyPI account** at [https://pypi.org/account/register/](https://pypi.org/account/register/) and enable 2FA. Then create an API token at [https://pypi.org/manage/account/token/](https://pypi.org/manage/account/token/).

Store the token in `~/.pypirc`:

```ini
[distutils]
index-servers = pypi

[pypi]
username = __token__
password = pypi-AgEI...YOUR_TOKEN_HERE
```

### 1.3 Prepare the Python SDK for Publication

The SDK lives in `sdks/python/`. The current `setup.py` is missing several fields required for a proper PyPI listing. Update it before publishing:

```python
# sdks/python/setup.py — complete version
from setuptools import setup, find_packages

setup(
    name="velarix",
    version="0.1.0",
    description="Python SDK for the Velarix belief graph and reasoning engine",
    long_description=open("README.md").read(),
    long_description_content_type="text/markdown",
    author="Velarix",
    author_email="team@velarix.dev",
    license="Apache-2.0",
    url="https://github.com/velarix/velarix",
    project_urls={
        "Documentation": "https://velarix.dev/docs",
        "Source": "https://github.com/velarix/velarix",
        "Bug Tracker": "https://github.com/velarix/velarix/issues",
    },
    packages=find_packages(),
    python_requires=">=3.9",
    install_requires=[
        "requests>=2.25.1",
        "httpx>=0.24.0",
        "openai>=1.0.0",
    ],
    extras_require={
        "langchain": [
            "langchain-core>=1.0.0",
            "langchain-openai>=1.0.0",
        ],
        "langgraph": [
            "langgraph>=0.3.0",
            "langgraph-checkpoint>=2.1.0",
            "langchain-core>=1.0.0",
            "langchain-openai>=1.0.0",
        ],
        "llamaindex": ["llama-index>=0.10.0"],
        "crewai": ["crewai>=0.108.0"],
        "local": [],
    },
    classifiers=[
        "Development Status :: 4 - Beta",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: Apache Software License",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
        "Programming Language :: Python :: 3.12",
        "Topic :: Software Development :: Libraries :: Python Modules",
    ],
)
```

You also need a `README.md` inside `sdks/python/` (the `long_description` reads it). Create a minimal one if it does not exist:

```bash
ls sdks/python/README.md || cp README.md sdks/python/README.md
```

### 1.4 Build and Publish the Python SDK

```bash
cd sdks/python

# Install build tools
pip install --upgrade pip build twine

# Build source distribution and wheel
python -m build
# Produces: dist/velarix-0.1.0.tar.gz
#           dist/velarix-0.1.0-py3-none-any.whl

# Verify the package metadata before uploading
twine check dist/*

# Publish to PyPI
twine upload dist/*

cd ../..
```

After this completes, `pip install velarix` will resolve to your package. The `velarix` namespace on PyPI is now owned by your account.

### 1.5 Claim the `velarix` Namespace on Docker Hub

Even though the production image will live in Google Artifact Registry, claiming the Docker Hub namespace prevents squatting and is needed for any `docker pull velarix/velarix` references in documentation.

1. Create an account at [https://hub.docker.com/](https://hub.docker.com/).
2. Create an organisation named `velarix` at [https://hub.docker.com/orgs](https://hub.docker.com/orgs).
3. Create a repository named `velarix` under that org.

Optionally push a tagged image to establish the namespace:

```bash
docker tag \
  us-central1-docker.pkg.dev/velarix-prod/velarix/velarix:latest \
  velarix/velarix:0.1.0

docker login
docker push velarix/velarix:0.1.0
docker push velarix/velarix:latest
```

---

## 2. Prerequisites

### Google Cloud Project Setup

```bash
# Create the project
gcloud projects create velarix-prod --name="Velarix Production"
gcloud config set project velarix-prod

# Link billing (replace with your billing account ID)
gcloud billing projects link velarix-prod \
  --billing-account=XXXXXX-XXXXXX-XXXXXX

# Enable all required APIs
gcloud services enable \
  run.googleapis.com \
  artifactregistry.googleapis.com \
  sqladmin.googleapis.com \
  redis.googleapis.com \
  secretmanager.googleapis.com \
  vpcaccess.googleapis.com \
  servicenetworking.googleapis.com \
  compute.googleapis.com \
  cloudbuild.googleapis.com
```

### Set shell variables used throughout this guide

```bash
export PROJECT_ID=velarix-prod
export REGION=us-central1
export REPO=velarix
export SERVICE_NAME=velarix
export FRONTEND_SERVICE_NAME=velarix-web
export SA_EMAIL="${SERVICE_NAME}-sa@${PROJECT_ID}.iam.gserviceaccount.com"
```

---

## 3. Containerisation

The `Dockerfile` at the repo root is a two-stage Alpine build. It compiles a statically linked Go binary — no CGO, no shared libraries required at runtime.

```dockerfile
# Dockerfile (from repo root — shown for reference)
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o velarix .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/velarix .
EXPOSE 8080
CMD ["./velarix", "--lite"]
```

**Important:** The default `CMD` starts the server with `--lite` (no auth, local BadgerDB only). In production, this flag must be absent. You override it at deployment time by setting `--command=""` and `--args="./velarix"` in the Cloud Run deploy command — do not modify the Dockerfile.

No build-time `ARG` or `ENV` values are needed. All configuration is injected as runtime environment variables.

### Create Artifact Registry and push

```bash
# Create the registry
gcloud artifacts repositories create velarix \
  --repository-format=docker \
  --location=$REGION \
  --description="Velarix container images"

# Configure Docker authentication
gcloud auth configure-docker ${REGION}-docker.pkg.dev

# Build the image (from the repo root)
docker build \
  -t ${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/velarix:$(git rev-parse --short HEAD) \
  -t ${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/velarix:latest \
  .

# Push both tags
docker push ${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/velarix:$(git rev-parse --short HEAD)
docker push ${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/velarix:latest
```

---

## 4. Cloud SQL (PostgreSQL)

The application uses the `pgx/v5` driver, reads `VELARIX_POSTGRES_DSN`, and runs embedded schema migrations on startup. No external migration tool is needed.

### Create the instance

```bash
gcloud sql instances create velarix-pg \
  --database-version=POSTGRES_16 \
  --tier=db-g1-small \
  --region=$REGION \
  --storage-type=SSD \
  --storage-size=20GB \
  --storage-auto-increase \
  --backup-start-time=03:00 \
  --deletion-protection
```

### Create the database and application user

```bash
gcloud sql databases create velarix --instance=velarix-pg

# Generate a strong password — you will store this in Secret Manager
DB_PASS=$(openssl rand -base64 32 | tr -d '\n')
echo "Save this password: $DB_PASS"

gcloud sql users create velarix \
  --instance=velarix-pg \
  --password="$DB_PASS"
```

### DSN format

The application reads `VELARIX_POSTGRES_DSN`. When connecting via the Cloud SQL Unix socket connector (injected by Cloud Run), the DSN uses a `host=` query parameter instead of a TCP host:

```
postgres://velarix:PASSWORD@/velarix?host=/cloudsql/velarix-prod:us-central1:velarix-pg
```

- `velarix` (first) — the database user created above
- `PASSWORD` — the value of `$DB_PASS`
- `/velarix` — the database name created above
- `host=` — the Unix socket path injected by Cloud Run when `--add-cloudsql-instances` is set

No `sslmode` parameter is needed for Unix socket connections.

The `.env.example` shows the TCP format for local development:

```
VELARIX_POSTGRES_DSN=postgres://user:password@localhost:5432/velarix?sslmode=disable
```

### Grant Cloud Run access to Cloud SQL

```bash
# Create the service account first (also used in subsequent steps)
gcloud iam service-accounts create ${SERVICE_NAME}-sa \
  --display-name="Velarix Cloud Run SA"

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/cloudsql.client"
```

---

## 5. Memorystore (Redis)

The application reads `VELARIX_REDIS_URL`. Redis is used for idempotency records (24-hour TTL) and rate limiting (2-minute window). The service starts and operates without Redis — it logs an error and falls back gracefully — but in production Redis should be present for correct multi-instance coordination.

Memorystore has no public IP. Cloud Run must reach it through a VPC connector.

### Create the VPC connector

```bash
gcloud compute networks vpc-access connectors create velarix-connector \
  --region=$REGION \
  --range=10.8.0.0/28
```

### Create the Redis instance

```bash
gcloud redis instances create velarix-redis \
  --size=1 \
  --region=$REGION \
  --redis-version=redis_7_0 \
  --tier=basic
```

### Retrieve the Redis address

```bash
REDIS_IP=$(gcloud redis instances describe velarix-redis \
  --region=$REGION \
  --format="value(host)")
REDIS_PORT=$(gcloud redis instances describe velarix-redis \
  --region=$REGION \
  --format="value(port)")

echo "Redis URL: redis://${REDIS_IP}:${REDIS_PORT}"
```

The Redis client in this codebase accepts `redis://host:port` (plain) or `rediss://host:port` (TLS). Memorystore basic tier does not use TLS, so use the `redis://` prefix.

---

## 6. Secret Manager

The following variables are secret-class and must never appear in plaintext in the Cloud Run service definition. Each becomes a named secret in Secret Manager and is referenced by name in the deploy command.

| Secret name | Variable it maps to | What it is |
|---|---|---|
| `VELARIX_ENCRYPTION_KEY` | `VELARIX_ENCRYPTION_KEY` | 32-character key for data-at-rest encryption |
| `VELARIX_JWT_SECRET` | `VELARIX_JWT_SECRET` | Signs console JWTs and decision execution tokens |
| `VELARIX_API_KEY` | `VELARIX_API_KEY` | Bootstrap admin bearer token |
| `VELARIX_POSTGRES_DSN` | `VELARIX_POSTGRES_DSN` | Full Postgres connection string including password |
| `VELARIX_REDIS_URL` | `VELARIX_REDIS_URL` | VPC-internal Redis address |
| `VELARIX_SMTP_PASS` | `VELARIX_SMTP_PASS` | SMTP password |
| `STRIPE_SECRET_KEY` | `STRIPE_SECRET_KEY` | Stripe live secret key |
| `STRIPE_WEBHOOK_SECRET` | `STRIPE_WEBHOOK_SECRET` | Stripe webhook signature secret |
| `OPENAI_API_KEY` | `OPENAI_API_KEY` | OpenAI API key for extraction and consistency verification |

### Create each secret

```bash
# VELARIX_ENCRYPTION_KEY — must be exactly 32 characters
echo -n "$(openssl rand -base64 24 | tr -d '=+/' | head -c 32)" | \
  gcloud secrets create VELARIX_ENCRYPTION_KEY --data-file=-

# VELARIX_JWT_SECRET
echo -n "$(openssl rand -base64 64)" | \
  gcloud secrets create VELARIX_JWT_SECRET --data-file=-

# VELARIX_API_KEY — bootstrap admin bearer token
echo -n "$(openssl rand -base64 48)" | \
  gcloud secrets create VELARIX_API_KEY --data-file=-

# VELARIX_POSTGRES_DSN — substitute your actual DB_PASS value
echo -n "postgres://velarix:${DB_PASS}@/velarix?host=/cloudsql/velarix-prod:us-central1:velarix-pg" | \
  gcloud secrets create VELARIX_POSTGRES_DSN --data-file=-

# VELARIX_REDIS_URL — substitute your actual REDIS_IP and REDIS_PORT values
echo -n "redis://${REDIS_IP}:${REDIS_PORT}" | \
  gcloud secrets create VELARIX_REDIS_URL --data-file=-

# VELARIX_SMTP_PASS — your SMTP provider password
echo -n "YOUR_SMTP_PASSWORD" | \
  gcloud secrets create VELARIX_SMTP_PASS --data-file=-

# STRIPE_SECRET_KEY
echo -n "sk_live_..." | \
  gcloud secrets create STRIPE_SECRET_KEY --data-file=-

# STRIPE_WEBHOOK_SECRET
echo -n "whsec_..." | \
  gcloud secrets create STRIPE_WEBHOOK_SECRET --data-file=-

# OPENAI_API_KEY
echo -n "sk-..." | \
  gcloud secrets create OPENAI_API_KEY --data-file=-
```

### Grant the service account access to every secret

```bash
for SECRET in \
  VELARIX_ENCRYPTION_KEY \
  VELARIX_JWT_SECRET \
  VELARIX_API_KEY \
  VELARIX_POSTGRES_DSN \
  VELARIX_REDIS_URL \
  VELARIX_SMTP_PASS \
  STRIPE_SECRET_KEY \
  STRIPE_WEBHOOK_SECRET \
  OPENAI_API_KEY; do
  gcloud secrets add-iam-policy-binding $SECRET \
    --member="serviceAccount:${SA_EMAIL}" \
    --role="roles/secretmanager.secretAccessor"
done
```

---

## 7. Cloud Run Deployment

### Port

The server reads the `PORT` environment variable and defaults to `8080` (see `api/server.go`). Cloud Run sets `PORT=8080` automatically. Do not override it.

### Non-secret environment variables

These are safe to set in plaintext in the service definition:

| Variable | Value | Source |
|---|---|---|
| `VELARIX_ENV` | `prod` | Enables HSTS, strict runtime checks |
| `VELARIX_STORE_BACKEND` | `postgres` | Switches from default BadgerDB to Postgres |
| `VELARIX_TRUST_PROXY_HEADERS` | `true` | Cloud Run sits behind Google's GFE proxy |
| `VELARIX_ENABLE_BOOTSTRAP_ADMIN_KEY` | `true` | Enables the `VELARIX_API_KEY` for initial setup |
| `VELARIX_AUTH_COOKIE_DOMAIN` | `velarix.dev` | Auth cookie domain |
| `VELARIX_AUTH_COOKIE_SAMESITE` | `Strict` | Default, but explicit |
| `VELARIX_BASE_URL` | `https://velarix.dev` | Password reset links and auth flows |
| `VELARIX_ALLOWED_ORIGINS` | `https://velarix.dev,https://www.velarix.dev` | CORS allowlist |
| `VELARIX_SMTP_ADDR` | `smtp.sendgrid.net:587` | SMTP host:port (adjust for your provider) |
| `VELARIX_SMTP_USER` | `apikey` | SendGrid SMTP username (adjust for your provider) |
| `VELARIX_SMTP_FROM` | `noreply@velarix.dev` | From address for password reset emails |
| `VELARIX_VERIFIER_MODEL` | `gpt-4o-mini` | OpenAI model for consistency verification |
| `VELARIX_IDEMPOTENCY_TTL_HOURS` | `24` | Matches the default in `.env.example` |
| `VELARIX_ADMIN_EMAIL` | `admin@velarix.dev` | Initial admin identity for bootstrap |

### Full deploy command

```bash
gcloud run deploy velarix \
  --image=${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/velarix:latest \
  --region=$REGION \
  --service-account=${SA_EMAIL} \
  --port=8080 \
  --memory=1Gi \
  --cpu=1 \
  --concurrency=100 \
  --min-instances=1 \
  --max-instances=10 \
  --timeout=300 \
  --add-cloudsql-instances=velarix-prod:us-central1:velarix-pg \
  --vpc-connector=velarix-connector \
  --vpc-egress=private-ranges-only \
  --no-allow-unauthenticated \
  --command="" \
  --args="./velarix" \
  --set-env-vars="VELARIX_ENV=prod" \
  --set-env-vars="VELARIX_STORE_BACKEND=postgres" \
  --set-env-vars="VELARIX_TRUST_PROXY_HEADERS=true" \
  --set-env-vars="VELARIX_ENABLE_BOOTSTRAP_ADMIN_KEY=true" \
  --set-env-vars="VELARIX_AUTH_COOKIE_DOMAIN=velarix.dev" \
  --set-env-vars="VELARIX_AUTH_COOKIE_SAMESITE=Strict" \
  --set-env-vars="VELARIX_BASE_URL=https://velarix.dev" \
  --set-env-vars="VELARIX_ALLOWED_ORIGINS=https://velarix.dev,https://www.velarix.dev" \
  --set-env-vars="VELARIX_SMTP_ADDR=smtp.sendgrid.net:587" \
  --set-env-vars="VELARIX_SMTP_USER=apikey" \
  --set-env-vars="VELARIX_SMTP_FROM=noreply@velarix.dev" \
  --set-env-vars="VELARIX_VERIFIER_MODEL=gpt-4o-mini" \
  --set-env-vars="VELARIX_IDEMPOTENCY_TTL_HOURS=24" \
  --set-env-vars="VELARIX_ADMIN_EMAIL=admin@velarix.dev" \
  --set-secrets="VELARIX_ENCRYPTION_KEY=VELARIX_ENCRYPTION_KEY:latest" \
  --set-secrets="VELARIX_JWT_SECRET=VELARIX_JWT_SECRET:latest" \
  --set-secrets="VELARIX_API_KEY=VELARIX_API_KEY:latest" \
  --set-secrets="VELARIX_POSTGRES_DSN=VELARIX_POSTGRES_DSN:latest" \
  --set-secrets="VELARIX_REDIS_URL=VELARIX_REDIS_URL:latest" \
  --set-secrets="VELARIX_SMTP_PASS=VELARIX_SMTP_PASS:latest" \
  --set-secrets="STRIPE_SECRET_KEY=STRIPE_SECRET_KEY:latest" \
  --set-secrets="STRIPE_WEBHOOK_SECRET=STRIPE_WEBHOOK_SECRET:latest" \
  --set-secrets="OPENAI_API_KEY=OPENAI_API_KEY:latest"
```

**Key flags explained:**

| Flag | Reason |
|---|---|
| `--port=8080` | Matches `api/server.go` default; Cloud Run health-checks this port |
| `--command="" --args="./velarix"` | Overrides the Dockerfile `CMD ["./velarix", "--lite"]` — omits `--lite` in production |
| `--add-cloudsql-instances` | Injects a Unix socket at `/cloudsql/velarix-prod:us-central1:velarix-pg`, which the `VELARIX_POSTGRES_DSN` `host=` path points to |
| `--vpc-connector` | Required to reach Memorystore — it has no public IP |
| `--vpc-egress=private-ranges-only` | Private traffic (Redis, Cloud SQL) goes through VPC; outbound to OpenAI goes direct |
| `--no-allow-unauthenticated` | Cloud Run IAM gate; public access is granted separately via domain mapping |
| `VELARIX_TRUST_PROXY_HEADERS=true` | Cloud Run receives requests via Google's GFE; this is required for accurate client IPs in rate limiting and logs |

### Allow public access

```bash
gcloud run services add-iam-policy-binding velarix \
  --region=$REGION \
  --member="allUsers" \
  --role="roles/run.invoker"
```

---

## 8. Custom Domain

### Map `velarix.dev` to the backend service

```bash
# Verify domain ownership (one-time; follow the console prompt)
gcloud domains verify velarix.dev

# Map the apex domain to the backend
gcloud run domain-mappings create \
  --service=velarix \
  --domain=velarix.dev \
  --region=$REGION
```

### DNS records

After creating the mapping, retrieve the DNS records Cloud Run requires:

```bash
gcloud run domain-mappings describe \
  --domain=velarix.dev \
  --region=$REGION
```

The output will list `A` records (IPv4), `AAAA` records (IPv6), or a `CNAME`. Add all of them to your DNS provider. Until DNS propagates, the domain will not route to Cloud Run.

### TLS

**Cloud Run handles TLS automatically.** It provisions and renews a Google-managed certificate for `velarix.dev` with no action required. The Go server binds plain HTTP on port 8080. Google's Front End terminates TLS before the request reaches the container.

There are no TLS certificate paths, private key files, or port 443 listeners anywhere in this codebase.

The codebase's own contribution to TLS posture is in `securityHeadersMiddleware` (`api/server.go`), which sets:

```
Strict-Transport-Security: max-age=31536000; includeSubDomains
```

This header is emitted on every response when `VELARIX_ENV=prod` (i.e., when `isDevLikeEnv()` returns `false`). It instructs browsers to always use HTTPS for one year, including all subdomains.

---

## 9. Web Frontend

The Next.js frontend in `web/` is a **separate service**. It has its own `package.json` (`"name": "velarix-web"`, `"version": "0.1.0"`), its own build process, and connects to the backend only via the `NEXT_PUBLIC_VELARIX_API_URL` environment variable baked in at build time. There is no shared binary with the Go server.

### Add `output: 'standalone'` to Next.js config

The existing `web/next.config.js` is empty. Enable standalone output for a self-contained Docker image:

```js
// web/next.config.js
/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
}
module.exports = nextConfig
```

### Create `web/Dockerfile`

There is no `web/Dockerfile` in the repository. Create it:

```dockerfile
# web/Dockerfile
FROM node:20-alpine AS builder
WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm ci

COPY . .

ENV NEXT_TELEMETRY_DISABLED=1

# These are baked into the static bundle at build time (Next.js requirement).
# They are not secrets — do not put them in Secret Manager.
ARG NEXT_PUBLIC_VELARIX_API_URL
ARG NEXT_PUBLIC_STRIPE_PUBLISHABLE_KEY
ENV NEXT_PUBLIC_VELARIX_API_URL=$NEXT_PUBLIC_VELARIX_API_URL
ENV NEXT_PUBLIC_STRIPE_PUBLISHABLE_KEY=$NEXT_PUBLIC_STRIPE_PUBLISHABLE_KEY

RUN npm run build

FROM node:20-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public 2>/dev/null || true

EXPOSE 3000
ENV PORT=3000
CMD ["node", "server.js"]
```

### Build and push the frontend image

```bash
cd web

docker build \
  --build-arg NEXT_PUBLIC_VELARIX_API_URL=https://velarix.dev \
  --build-arg NEXT_PUBLIC_STRIPE_PUBLISHABLE_KEY=pk_live_... \
  -t ${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/velarix-web:$(git -C .. rev-parse --short HEAD) \
  -t ${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/velarix-web:latest \
  .

docker push ${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/velarix-web:$(git -C .. rev-parse --short HEAD)
docker push ${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/velarix-web:latest

cd ..
```

`NEXT_PUBLIC_*` variables are embedded into the JavaScript bundle at build time. They are not secrets and must not go into Secret Manager. Pass them as `--build-arg` values.

### Deploy the frontend

```bash
gcloud run deploy velarix-web \
  --image=${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/velarix-web:latest \
  --region=$REGION \
  --port=3000 \
  --memory=512Mi \
  --cpu=1 \
  --concurrency=200 \
  --min-instances=0 \
  --max-instances=5 \
  --allow-unauthenticated
```

### Map `www.velarix.dev` to the frontend

```bash
gcloud run domain-mappings create \
  --service=velarix-web \
  --domain=www.velarix.dev \
  --region=$REGION
```

Add the DNS records Cloud Run returns to your DNS provider.

---

## 10. CI/CD

The existing `.github/workflows/publish.yml` handles only PyPI and NPM SDK publishing — it does not build or deploy the Go backend or the Next.js frontend.

Create `.github/workflows/deploy.yml`:

```yaml
name: Build and Deploy to Cloud Run

on:
  push:
    branches: [main]

env:
  PROJECT_ID: velarix-prod
  REGION: us-central1
  REPO: velarix
  SERVICE_NAME: velarix
  FRONTEND_SERVICE_NAME: velarix-web

jobs:
  deploy-backend:
    name: Build and deploy backend
    runs-on: ubuntu-latest
    permissions:
      contents: read
      id-token: write   # Required for Workload Identity Federation

    steps:
      - uses: actions/checkout@v4

      - name: Authenticate to Google Cloud
        uses: google-github-actions/auth@v2
        with:
          # Replace PROJECT_NUMBER with the numeric ID of velarix-prod
          workload_identity_provider: >-
            projects/PROJECT_NUMBER/locations/global/workloadIdentityPools/github/providers/github
          service_account: github-deploy@velarix-prod.iam.gserviceaccount.com

      - name: Set up Cloud SDK
        uses: google-github-actions/setup-gcloud@v2

      - name: Configure Docker for Artifact Registry
        run: gcloud auth configure-docker ${{ env.REGION }}-docker.pkg.dev --quiet

      - name: Build and push backend image
        run: |
          IMAGE="${{ env.REGION }}-docker.pkg.dev/${{ env.PROJECT_ID }}/${{ env.REPO }}/velarix:${{ github.sha }}"
          docker build -t "$IMAGE" .
          docker push "$IMAGE"
          docker tag "$IMAGE" \
            "${{ env.REGION }}-docker.pkg.dev/${{ env.PROJECT_ID }}/${{ env.REPO }}/velarix:latest"
          docker push \
            "${{ env.REGION }}-docker.pkg.dev/${{ env.PROJECT_ID }}/${{ env.REPO }}/velarix:latest"

      - name: Deploy backend to Cloud Run
        run: |
          gcloud run deploy ${{ env.SERVICE_NAME }} \
            --image="${{ env.REGION }}-docker.pkg.dev/${{ env.PROJECT_ID }}/${{ env.REPO }}/velarix:${{ github.sha }}" \
            --region=${{ env.REGION }} \
            --quiet

  deploy-frontend:
    name: Build and deploy frontend
    runs-on: ubuntu-latest
    needs: deploy-backend   # frontend depends on backend being live first
    permissions:
      contents: read
      id-token: write

    steps:
      - uses: actions/checkout@v4

      - name: Authenticate to Google Cloud
        uses: google-github-actions/auth@v2
        with:
          workload_identity_provider: >-
            projects/PROJECT_NUMBER/locations/global/workloadIdentityPools/github/providers/github
          service_account: github-deploy@velarix-prod.iam.gserviceaccount.com

      - name: Set up Cloud SDK
        uses: google-github-actions/setup-gcloud@v2

      - name: Configure Docker for Artifact Registry
        run: gcloud auth configure-docker ${{ env.REGION }}-docker.pkg.dev --quiet

      - name: Build and push frontend image
        run: |
          IMAGE="${{ env.REGION }}-docker.pkg.dev/${{ env.PROJECT_ID }}/${{ env.REPO }}/velarix-web:${{ github.sha }}"
          docker build \
            --build-arg NEXT_PUBLIC_VELARIX_API_URL=https://velarix.dev \
            --build-arg NEXT_PUBLIC_STRIPE_PUBLISHABLE_KEY=${{ secrets.NEXT_PUBLIC_STRIPE_PUBLISHABLE_KEY }} \
            -t "$IMAGE" \
            web/
          docker push "$IMAGE"

      - name: Deploy frontend to Cloud Run
        run: |
          gcloud run deploy ${{ env.FRONTEND_SERVICE_NAME }} \
            --image="${{ env.REGION }}-docker.pkg.dev/${{ env.PROJECT_ID }}/${{ env.REPO }}/velarix-web:${{ github.sha }}" \
            --region=${{ env.REGION }} \
            --quiet
```

### Set up Workload Identity Federation

Key-based service account JSON is not recommended. Use Workload Identity Federation instead:

```bash
PROJECT_NUMBER=$(gcloud projects describe $PROJECT_ID --format="value(projectNumber)")

# Create the pool
gcloud iam workload-identity-pools create github \
  --location=global \
  --display-name="GitHub Actions"

# Create the OIDC provider
gcloud iam workload-identity-pools providers create-oidc github \
  --location=global \
  --workload-identity-pool=github \
  --display-name="GitHub" \
  --attribute-mapping="google.subject=assertion.sub,attribute.repository=assertion.repository" \
  --issuer-uri="https://token.actions.githubusercontent.com"

# Create the deploy service account
gcloud iam service-accounts create github-deploy \
  --display-name="GitHub Actions Deploy SA"

GITHUB_SA="github-deploy@${PROJECT_ID}.iam.gserviceaccount.com"

# Grant it the minimum permissions needed to deploy
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:${GITHUB_SA}" \
  --role="roles/run.admin"

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:${GITHUB_SA}" \
  --role="roles/artifactregistry.writer"

gcloud iam service-accounts add-iam-policy-binding $GITHUB_SA \
  --role="roles/iam.workloadIdentityUser" \
  --member="principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/github/attribute.repository/velarix/velarix"
```

### GitHub Actions secret to add

In your repository settings under **Settings → Secrets → Actions**, add:

| Secret | Value |
|---|---|
| `NEXT_PUBLIC_STRIPE_PUBLISHABLE_KEY` | `pk_live_...` |

---

## 11. Production Checklist

Run each check in order after the first full deployment.

### Health check endpoint responding

```bash
curl -s https://velarix.dev/health | jq .
```

Expected response:

```json
{
  "status": "healthy",
  "version": "0.1.0",
  "uptime": "..."
}
```

A non-200 response or connection error means the Cloud Run service did not start. Check logs:

```bash
gcloud run services logs read velarix --region=us-central1 --limit=50
```

If `VELARIX_ENCRYPTION_KEY` or `VELARIX_JWT_SECRET` are missing or malformed, the server will refuse to start in production mode. The logs will say so explicitly.

### Full health check (PostgreSQL connection live)

```bash
# Retrieve your bootstrap API key from Secret Manager
ADMIN_KEY=$(gcloud secrets versions access latest --secret=VELARIX_API_KEY)

curl -s https://velarix.dev/health/full \
  -H "Authorization: Bearer $ADMIN_KEY" | jq .
```

Expected response includes:

```json
{
  "status": "healthy",
  "storage_backend": "postgres",
  "storage_connected": true,
  ...
}
```

`storage_connected: false` means the Cloud SQL connection failed. Verify:
- The `--add-cloudsql-instances` flag matches the instance connection name (`velarix-prod:us-central1:velarix-pg`).
- The `VELARIX_POSTGRES_DSN` secret contains the correct `host=` path.
- The service account has the `roles/cloudsql.client` IAM binding.

### Metrics endpoint restricted

```bash
# Unauthenticated request must not return 200
curl -s -o /dev/null -w "%{http_code}" https://velarix.dev/metrics
```

Expected: `401` or `403`. Never `200` from an unauthenticated request. If it returns `200`, verify `VELARIX_ENV=prod` is set — the metrics handler applies auth middleware only in production mode.

### TLS active and HSTS header present

```bash
# Confirm TLS version and HTTP/2
curl -vI https://velarix.dev/health 2>&1 | grep -E "SSL|TLS|HTTP/"

# Confirm HSTS header
curl -sI https://velarix.dev/health | grep -i strict-transport-security
```

Expected HSTS value (set by `securityHeadersMiddleware` when `VELARIX_ENV=prod`):

```
strict-transport-security: max-age=31536000; includeSubDomains
```

### All secrets loading

```bash
# Inspect the running revision's environment
gcloud run services describe velarix \
  --region=us-central1 \
  --format="yaml(spec.template.spec.containers[0].env)"
```

All nine secret-class variables should appear with `valueFrom.secretKeyRef` references, not plaintext values.

### Redis reachable

Redis errors are non-fatal and logged. Check for connection errors in the logs:

```bash
gcloud run services logs read velarix --region=us-central1 --limit=100 | grep -i redis
```

No `dial tcp`, `connection refused`, or `timeout` entries expected. If present:
- Verify `--vpc-connector=velarix-connector` is on the service.
- Verify `--vpc-egress=private-ranges-only` so private IPs route through the connector.
- Verify the `VELARIX_REDIS_URL` secret contains the correct internal IP.

### Python SDK smoke test against live deployment

```bash
pip install velarix

# Retrieve the bootstrap API key
ADMIN_KEY=$(gcloud secrets versions access latest --secret=VELARIX_API_KEY)

python3 - <<EOF
import velarix

client = velarix.VelarixClient(
    base_url="https://velarix.dev",
    api_key="${ADMIN_KEY}"
)

# Open a session and assert a fact
session = client.session()
session.assert_fact("deployment", "status", "live")
facts = session.get_slice(format="json", max_facts=10)
print("Facts returned:", facts)
assert any(
    f.get("subject") == "deployment" for f in (facts if isinstance(facts, list) else [])
), "Fact not found in slice"

print("Smoke test passed — deployment is live and healthy.")
EOF
```

### Final service URLs

| Service | URL |
|---|---|
| Backend API | `https://velarix.dev` |
| Frontend | `https://www.velarix.dev` |
| Health (unauthenticated) | `https://velarix.dev/health` |
| Full health (admin only) | `https://velarix.dev/health/full` |
| Metrics (auth required) | `https://velarix.dev/metrics` |
| Prometheus scrape target | `https://velarix.dev/metrics` |

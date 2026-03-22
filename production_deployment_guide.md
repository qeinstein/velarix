# Production Deployment Guide: From Cloud-Hosted to Embedded

If you are used to cloud-hosted databases (Postgres, Mongo) where you just enter a URL, the Velarix **Embedded DB** model will feel different but simpler. 

## 1. The "Mental Shift"
- **No URL**: There is no connection string. The database is a local folder called `velarix.data`.
- **Zero-Dependency**: You don't need to spin up a RDS or Atlas instance. The engine *is* the database.
- **Data Aggregation**: Yes, all multi-tenant data (different organizations) is physically stored in the **same folder** on the server.
- **Logical Isolation**: We enforce strict isolation via `OrgID` on every API request to ensure Tenant A never sees Tenant B's data, despite being in the same physical KV store.
- **Portability**: To move your database, you literally just zip the `velarix.data` folder.

## 2. Production Checklist (Your "To-Do")

### 🛡️ Persistent Volumes (CRITICAL)
If you deploy via Docker, Kubernetes, or Render/Fly.io, the container's storage is ephemeral (wiped on restart). 
- **Requirement**: You **must** mount a persistent volume to the `/root/velarix.data` directory (as seen in the `Dockerfile`).
- **Example (Docker)**: `docker run -v /my/host/data:/root/velarix.data velarix-server`

### 🔑 Encryption Key
Instead of a database password, you manage a 32-byte encryption key.
- **Requirement**: Provide `VELARIX_ENCRYPTION_KEY` as an environment variable in your production environment.
- **Security**: This key encrypts the data at rest. If you lose this key, your data is unrecoverable.

### 💾 Backup Strategy
Since there is no "Cloud Backup" UI, you have two options:
1. **API Backup**: Call `GET /v1/org/backup` periodically and save the result to S3/GCS.
2. **Raw Backup**: Periodically zip the `velarix.data` folder (though the API is safer for consistency).

## 3. Example Deployment (Docker Compose)
```yaml
services:
  velarix:
    image: velarix-server
    ports:
      - "8080:8080"
    volumes:
      - ./prod_data:/root/velarix.data # This is your "database server"
    environment:
      - VELARIX_ENV=prod
      - VELARIX_ENCRYPTION_KEY=${VELARIX_ENCRYPTION_KEY} # 32-byte key
      - VELARIX_API_KEY=${ADMIN_API_KEY}
```

## 4. Where to Deploy?
Because Velarix requires **Persistent Storage** (a "Disk" or "Volume"), you should choose providers that support this:

- **Render / Fly.io (Recommended)**: Both support "Volumes." You can deploy the Docker image and mount a 1GB-10GB disk easily.
- **DigitalOcean / Linode (VPS)**: Excellent for this. Simply run the binary or Docker. The data stays on the instance's SSD.
- **AWS ECS (with EBS)**: If you need enterprise-grade, use ECS and mount an EBS volume for the `velarix.data` directory.
- **⚠️ Avoid**: Heroku (Standard) or early Vercel, as they use ephemeral filesystems that wipe your database on every deployment.

## 5. Why this way?
- **Speed**: No network overhead between the application and the database.
- **Durability**: Every write is synced to the machine's disk instantly.
- **Simplicity**: No "connection pooling" issues or "database connections exhausted" errors.

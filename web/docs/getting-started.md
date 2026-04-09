---
title: "Getting Started"
description: "Local startup and production prerequisites."
order: 2
---

# Getting Started

Velarix runs as a Go API with SDK clients and framework integrations on top.

## Prerequisites

- Go 1.23 or later for the API
- Python 3.10 or later for the SDK and demos
- Docker for containerized deployment

## Local API

Start the API from the project root:

```bash
export VELARIX_ENV=dev
export VELARIX_API_KEY=dev-admin-key
export VELARIX_BADGER_PATH="$(mktemp -d)"
go run main.go
```

## Python SDK

Install the SDK:

```bash
pip install -e ./sdks/python
```

Initialize a session:

```python
from velarix import VelarixClient

client = VelarixClient(
    base_url="http://localhost:8080",
    api_key="dev-admin-key",
)
session = client.session("my-first-session")
```

Record a fact:

```python
session.observe("user_authenticated", {"user_id": "123"})
```

Retrieve the current belief slice:

```python
session.get_slice(
    query="user authentication state",
    strategy="hybrid",
    include_dependencies=True,
)
```

## Production Path

Production deployments use:

- Postgres for runtime state
- Redis for shared coordination
- `VELARIX_JWT_SECRET` for console authentication
- `VELARIX_ALLOWED_ORIGINS` for browser access

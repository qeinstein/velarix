# Velarix Control Plane (Command Center)

This repository contains the private, operational logic for Velarix Cloud. This is separated from the core open-source reasoning engine to ensure the open-source code remains a pure developer tool, while this Control Plane handles the "unsexy" business logic required to run a profitable, secure SaaS.

## Architecture

*   **`billing/`**: Stripe webhooks, subscription logic, invoice generation, and tier enforcement (e.g., Free vs. Enterprise).
*   **`infrastructure/`**: Terraform/Pulumi scripts for deploying and scaling the high-performance Go reasoning clusters on AWS/GCP.
*   **`provisioning/`**: Automated tenant provisioning, database migrations for new Orgs, and Redis rate-limit configuration.

## The "Gatekeeper" Strategy

The open-source community runs the core Velarix engine locally for free.

When users access our hosted dashboard, it points to `NEXT_PUBLIC_VELARIX_API_URL=https://api.velarix.com`. The Control Plane intercepts these requests, enforces API key validation, checks Stripe subscription status, and then securely routes the authenticated requests to the internal reasoning clusters.

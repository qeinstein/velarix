# Velarix Control Plane

The control plane contains the hosted-service operational surfaces that sit alongside the reasoning API.

## Current Components

- `billing/`: Stripe webhook handling, subscription updates, and plan-derived feature state
- `infrastructure/`: deployment templates and infrastructure definitions
- `provisioning/`: tenant setup and environment bootstrap helpers

## Current Shape

The current scope covers:

- billing-state ingestion
- deployment scaffolding
- tenant setup workflows

## Product Boundary

The public API remains the product surface for reasoning, decisions, and execution integrity.

The control plane supports provisioning and service operations around that product surface.

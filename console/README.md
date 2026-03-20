# Velarix Control Plane

A world-class observability workspace for clinical reasoning and belief graphs. 

The control plane allows operators to visualize the causal dependencies of an AI agent, replay reasoning history, and predict the blast radius of clinical premised retractions.

## Features

- **3D Exploded-View Architecture**: Visualizes the App, Kernel, and Storage layers with parallax depth.
- **Living Neural Graph**: Nodes pulse with health status; edges animate flowing causal particles.
- **Time-Travel Debugging**: Journal-driven simulation mode to scrub through reasoning cascades.
- **Compliance Center**: One-click SHA-256 verified SOC2/HIPAA audit log exports.
- **Blame & Provenance**: Full chronological lineage for every belief, including which LLM model provided the justification.

## Technical Stack

- **Framework**: React 19 + TypeScript.
- **Visuals**: Framer Motion (Interactions), React Flow (Graph).
- **Styling**: Vanilla CSS with Vercel/Linear-inspired aesthetics.
- **Hardening**: Versioned API integration (`/v1`), strict tenant isolation, and snake_case payloads aligned with the backend contract.

## Getting Started

```bash
npm install
npm run dev
```

---
*Velarix: Auditable reasoning for the regulated clinical plane.*

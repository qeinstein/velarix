# Velarix TypeScript SDK

Minimal TypeScript client for interacting with Velarix sessions.

## Install

```bash
npm i
```

## Usage

```ts
import { VelarixClient, VelarixGateway } from "./src";

const client = new VelarixClient({ baseUrl: "http://localhost:8080", apiKey: "vx_..." });
const session = client.session("s_demo");

// Writes automatically include Idempotency-Key to make retries safe.
await session.observe("patient.intake", { mrn: "123" });

// Record a decision record event (stored in session history).
await session.recordDecision({ kind: "tool_call", tool: "eligibility.check", input: { mrn: "123" } });

// Gateway helper: wrap real tool calls and emit decision records automatically.
const gateway = new VelarixGateway(session);
await gateway.callTool("eligibility.check", { mrn: "123" }, async (input) => ({ eligible: true }));
```

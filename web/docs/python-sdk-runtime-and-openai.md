---
title: "Python SDK: Runtime And OpenAI Adapter"
description: "Use the chat runtime and OpenAI-compatible adapter to inject Velarix context into model calls and optionally verify model outputs against the belief graph."
section: "Python SDK"
sectionOrder: 4
order: 2
---

# Runtime and OpenAI adapter

This page covers `sdks/python/velarix/runtime.py` and `sdks/python/velarix/adapters/openai.py`. These modules are not part of the symbolic hot path in the server. They are client-side orchestration helpers for LLM applications.

## `VelarixChatRuntime`

`VelarixChatRuntime` wraps a session and exposes helpers that:

- fetch a ranked session slice and inject it into prompts
- expose a stable tool set for agent loops
- record observations and reasoning artifacts back into Velarix
- optionally verify recent reasoning output

Tool names exported by the runtime:

- `record_observation`
- `explain_reasoning`
- `record_perception`
- `semantic_memory_search`
- `record_reasoning_chain`
- `verify_reasoning_chain`
- `consistency_check`

Important behavior:

- the runtime converts a session slice into markdown and appends it to the system prompt
- `prepare_params(...)` builds tool definitions and prompt context
- `process_response(...)` records new facts and reasoning artifacts from model output
- `verify_recent_reasoning(...)` can run post-response verification
- if a root observation is recorded with confidence greater than `0.9`, the runtime caps it to `0.75` and writes a `confidence_adjusted` history event through `append_history()`

That last point means the adjustment log only works on deployments that expose a history-write route. The default Go router does not.

Example:

```python
from velarix.client import VelarixClient
from velarix.runtime import VelarixChatRuntime

client = VelarixClient(base_url="http://localhost:8080", api_key="vxk_test")
session = client.session("runtime-demo")
runtime = VelarixChatRuntime(session)

session.observe("vendor_verified", {"vendor_id": "vendor-17"})
session.observe("invoice_approved", {"invoice_id": "inv-1042"})

params = runtime.prepare_params(
    system_prompt="Answer using only justified facts when possible.",
    user_prompt="Can I release payment for invoice inv-1042?",
)

print(params["messages"][0]["content"])
print([tool["function"]["name"] for tool in params["tools"]])
```

## `AsyncVelarixChatRuntime`

The async runtime mirrors the synchronous API for async model pipelines.

Use it when your application already uses `asyncio`, `httpx`, or an async LLM SDK.

## OpenAI adapter

`sdks/python/velarix/adapters/openai.py` exposes wrappers that mimic the OpenAI client surface while injecting Velarix session behavior.

Classes:

- `OpenAI`
- `VelarixChat`
- `VelarixCompletions`
- `AsyncOpenAI`
- `VelarixAsyncChat`
- `VelarixAsyncCompletions`

Configuration:

- `velarix_api_key` argument or `VELARIX_API_KEY`
- `base_url` argument or `VELARIX_BASE_URL`
- per-call options:
  - `velarix_session_id`
  - `velarix_strict`
  - `velarix_verify_rounds`
  - `velarix_auto_verify`

Behavior:

- the adapter creates or reuses a Velarix session for the request
- it injects session context into the model call
- if verification fails and auto-verification is enabled, it appends a corrective user message and retries up to `velarix_verify_rounds`

Example:

```python
from velarix.adapters.openai import OpenAI

client = OpenAI(
    api_key="sk-openai",
    velarix_api_key="vxk_test",
    base_url="https://api.openai.com/v1",
)

response = client.chat.completions.create(
    model="gpt-4.1-mini",
    messages=[
        {"role": "system", "content": "Answer carefully."},
        {"role": "user", "content": "Summarize whether payment can be released."},
    ],
    velarix_session_id="invoice-demo",
    velarix_auto_verify=True,
    velarix_verify_rounds=2,
)

print(response.choices[0].message.content)
```

## When to use the runtime vs the adapter

- Use `VelarixChatRuntime` when you already control the model client and want explicit access to the tool definitions, prompt augmentation, and response-processing steps.
- Use the OpenAI adapter when you want an OpenAI-like interface with minimal call-site changes.

## Limitation to document explicitly

Neither helper changes the server’s reasoning model. The server still evaluates facts, justifications, contradictions, and decisions symbolically. These Python modules only help LLM applications interact with that server.

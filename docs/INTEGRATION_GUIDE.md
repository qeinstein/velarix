# Integration Guide

Velarix should be integrated as a pre-execution guardrail, not as a generic agent-memory library.

The most important pattern is:

1. collect the approval facts
2. create a decision
3. call `execute-check` immediately before the action
4. call `execute` only if the decision is still executable

## Recommended Integration Pattern

Velarix works best when it sits in front of an existing execution system such as:

- payment release workflow
- refund approval flow
- invoice exception path
- internal access approval action

Your application remains the system that performs the action.

Velarix determines whether the action is still safe to perform.

## Python SDK Example

```python
from velarix.client import VelarixClient

client = VelarixClient(base_url="http://localhost:8080", api_key="your_key")
session = client.session("payment_approval_001")

session.observe("vendor_verified", {"vendor_id": "vendor-17"})
session.observe("invoice_approved", {"invoice_id": "inv-1042"})
session.observe("budget_available", {"cost_center": "ENG-01"})

session.derive(
    "decision.release_payment",
    [["vendor_verified", "invoice_approved", "budget_available"]],
    {"summary": "Payment can be released"}
)

decision = session.create_decision(
    "payment_release",
    fact_id="decision.release_payment",
    subject_ref="inv-1042",
    target_ref="vendor-17",
    dependency_fact_ids=["vendor_verified", "invoice_approved", "budget_available"],
)

check = session.execute_check(decision["decision_id"])
if check["executable"]:
    session.execute_decision(decision["decision_id"])
```

## Execution-Control Pattern

Use this pattern in production:

- Velarix never silently performs the business action
- your application remains the execution owner
- Velarix decides whether the approval is still valid

This keeps the product focused on decision integrity.

## OpenAI Adapter

The OpenAI adapter exists and can still be used.

It should be treated as a secondary integration surface, not the product identity.

Use it when you want model-emitted observations to flow into the same approval session.

## Framework Integrations

LangChain, LangGraph, and LlamaIndex integrations remain exploratory surfaces in this repo.

They are useful only if they feed or consume the same approval-guardrail workflow.

They are not the primary buyer story.

## Design Rule

If an integration does not sit close to a real execution step, it is secondary.

The product is strongest when it can block a stale internal approval before money, access, or operational state changes.


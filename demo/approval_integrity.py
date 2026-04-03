import json
import os
import sys
import time

import requests

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../sdks/python")))

from velarix import VelarixClient


def require_env(name: str) -> str:
    value = os.getenv(name, "").strip()
    if value:
        return value
    raise RuntimeError(f"{name} is required")


def print_header(title: str) -> None:
    print(f"\n{'=' * 72}")
    print(title)
    print(f"{'=' * 72}")


def format_fact_line(fact: dict) -> str:
    payload = fact.get("payload") or {}
    summary = payload.get("summary")
    if not summary:
        summary = json.dumps(payload, sort_keys=True)
    return f"- {fact.get('id')}: {summary} (status={fact.get('resolved_status')})"


def print_valid_slice(session, title: str) -> None:
    print_header(title)
    facts = session.get_slice(format="json", max_facts=20)
    if not facts:
        print("(no valid facts)")
        return
    for fact in facts:
        print(format_fact_line(fact))


def print_recent_history(session) -> None:
    print_header("Recent Session History")
    history = session.get_history()
    for entry in history[-5:]:
        payload = entry.get("payload") or {}
        print(
            f"- {entry.get('type')} "
            f"fact_id={entry.get('fact_id') or payload.get('fact_id') or '-'} "
            f"actor={entry.get('actor_id') or '-'}"
        )


def main() -> None:
    base_url = os.getenv("VELARIX_BASE_URL", "http://localhost:8080").rstrip("/")
    api_key = require_env("VELARIX_API_KEY")
    session_id = os.getenv("VELARIX_SESSION_ID", f"approval_integrity_demo_{int(time.time())}")

    requests.get(f"{base_url}/health", timeout=2).raise_for_status()

    client = VelarixClient(base_url=base_url, api_key=api_key)
    session = client.session(session_id)

    print_header("Approval Integrity Demo")
    print(f"Base URL:   {base_url}")
    print(f"Session ID: {session_id}")

    print("\nRecording approval inputs...")
    session.observe(
        "vendor_profile_verified",
        {
            "summary": "Vendor onboarding is complete for vendor-17",
            "vendor_id": "vendor-17",
        },
    )
    session.observe(
        "invoice_approved",
        {
            "summary": "Accounts payable approved invoice inv-1042 for $25,000",
            "invoice_id": "inv-1042",
            "amount_usd": 25000,
        },
    )
    session.observe(
        "budget_available",
        {
            "summary": "Budget is available in cost center ENG-01",
            "cost_center": "ENG-01",
            "available_amount_usd": 50000,
        },
    )

    print("Deriving the AI recommendation...")
    session.derive(
        "decision.release_vendor_payment",
        [["vendor_profile_verified", "invoice_approved", "budget_available"]],
        {
            "summary": "AI recommends releasing payment for inv-1042",
            "recommended_action": "release_payment",
            "subject_ref": "inv-1042",
            "target_ref": "vendor-17",
        },
    )
    decision = session.create_decision(
        "approval_recommendation",
        fact_id="decision.release_vendor_payment",
        recommended_action="release_payment",
        subject_ref="inv-1042",
        target_ref="vendor-17",
        dependency_fact_ids=["vendor_profile_verified", "invoice_approved", "budget_available"],
        metadata={"workflow": "approval_integrity_demo"},
    )
    decision_id = decision["decision_id"]

    print_valid_slice(session, "Valid Context Before Fact Change")

    check = session.execute_check(decision_id)
    print("\nExecution check before change:")
    print(f"- decision_id={decision_id}")
    print(f"- executable={check.get('executable')}")
    print(f"- reason_codes={check.get('reason_codes')}")

    print("\nUpstream fact changes: budget approval is withdrawn.")
    session.invalidate("budget_available")

    print_valid_slice(session, "Valid Context After Fact Change")

    blocked = session.execute_check(decision_id)
    print("\nExecution check after change:")
    print(f"- executable={blocked.get('executable')}")
    print(f"- reason_codes={blocked.get('reason_codes')}")
    if not blocked.get("executable", False):
        print("- execution is blocked because the recommendation is now stale")
    else:
        print("- recommendation is still executable")

    try:
        session.execute_decision(decision_id, execution_ref="demo_run_after_budget_withdrawal")
    except requests.HTTPError as exc:
        response = exc.response.json() if exc.response is not None else {"error": str(exc)}
        print_header("Blocked Execute Response")
        print(json.dumps(response, indent=2))

    try:
        explanation = session.get_decision_why_blocked(decision_id)
        print_header("Why Blocked")
        print(json.dumps(explanation, indent=2))
    except requests.RequestException as exc:
        print(f"\nExplanation request failed: {exc}")

    print_recent_history(session)


if __name__ == "__main__":
    main()

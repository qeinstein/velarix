import time
import sys
import os
import requests

# Ensure the sdks path is in the PYTHONPATH so we can import velarix
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '../sdks/python')))

from velarix import VelarixClient

def print_context(client: VelarixClient, title: str):
    print(f"\n{'='*50}")
    print(f" AGENT MEMORY CONTEXT: {title}")
    print(f"{'='*50}")
    
    context = client.get_valid_context()
    
    if not context:
        print("   (Memory is empty)")
    else:
        for fact in context:
            # Check if the fact is valid before printing
            if fact.get("resolved_status") == 1: # 1 == Valid
                prefix = " Root Premise:" if fact.get("IsRoot") else "↳ Derived Plan:"
                payload = fact.get("payload", {})
                desc = payload.get("desc") or payload.get("action") or "Unknown"
                print(f"   {prefix} [{fact.get('ID')}] {desc}")
    
    print(f"{'='*50}\n")


def run_demo():
    print(" Initializing Agent Framework with Velarix Context Firewall...")
    client = VelarixClient(base_url="http://localhost:8080")

    # PHASE A: The Observation
    print("\n[AGENT] Scanning repository for dependencies...")
    time.sleep(1)
    client.observe("premise_react17", {"desc": "Package.json specifies React 17"})
    print(" [VELARIX] Stored Root Premise: Project uses React 17")

    # PHASE B: The Chain of Thought (Hallucination Build-up)
    print("\n[AGENT] Generating execution plan based on React 17...")
    time.sleep(1)
    
    client.derive(
        "plan_step_1", 
        justifications=[["premise_react17"]], 
        payload={"action": "Use ReactDOM.render() for entrypoint"}
    )
    print("   ↳ Generated Step 1: Use ReactDOM.render()")
    
    client.derive(
        "plan_step_2", 
        justifications=[["plan_step_1"]], 
        payload={"action": "Avoid Server Components (not supported in 17)"}
    )
    print("   ↳ Generated Step 2: Avoid Server Components")
    
    client.derive(
        "plan_step_3", 
        justifications=[["plan_step_2"]], 
        payload={"action": "Build custom hook for state management"}
    )
    print("   ↳ Generated Step 3: Build custom hook")

    print_context(client, "BEFORE INTERRUPTION")

    # PHASE C: The Interruption
    print(" [USER INPUT INTERRUPT] 'Wait, actually let's use React 18!'")
    time.sleep(1)
    print(" [VELARIX] Invalidating premise: premise_react17...")
    client.invalidate("premise_react17")
    
    # PHASE D: The Velarix Collapse
    print(" [VELARIX] O(1) Dominator Pruning activated. Collapsing dependent reasoning...")
    time.sleep(1)
    
    print_context(client, "AFTER VELARIX COLLAPSE")

    # PHASE E: The Pivot
    print("[AGENT] Re-evaluating based on new reality...")
    time.sleep(1)
    client.observe("premise_react18", {"desc": "User requested React 18"})
    
    client.derive(
        "plan_step_1_new", 
        justifications=[["premise_react18"]], 
        payload={"action": "Use createRoot() for entrypoint"}
    )
    print("   ↳ Generated New Step 1: Use createRoot()")
    
    client.derive(
        "plan_step_2_new", 
        justifications=[["plan_step_1_new"]], 
        payload={"action": "Utilize React Server Components"}
    )
    print("   ↳ Generated New Step 2: Utilize Server Components")
    
    print_context(client, "FINAL AGILE PIVOT")
    print(" Demo Complete. Zero Hallucinations. Stale context instantly purged.")

if __name__ == "__main__":
    try:
        # A simple check to see if the server is running
        requests.get("http://localhost:8080/facts")
        run_demo()
    except Exception as e:
        print(f" Error: {e}")
        print("Please ensure the Go server is running on http://localhost:8080")
        print("Run `go run main.go` in the root directory first.")

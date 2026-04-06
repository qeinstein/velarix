# The Science of Velarix

Velarix is a Neuro-Symbolic Hybrid Truth Maintenance System (TMS). It bridges the gap between the probabilistic, noisy world of LLMs and the deterministic, causal world of symbolic logic.

## 1. The Dominator Tree Advantage
In a standard Directed Acyclic Graph (DAG), if you delete a node, you have to traverse the entire graph to find its children. In an **AND/OR graph** (which agents use to reason: *I believe Z because of (A AND B) OR (C)*), this is complex.

Velarix reduces this complexity to an **Ancestor Check** using a **Dominator Tree**. If Root Fact $A$ dominates Fact $Z$, and $A$ becomes invalid, we know $Z$ is invalid in $O(1)$ time. This is the only way to scale agent memory to 100,000+ facts without slowing down inference.

## 2. Epistemic Confidence Capping
Agents are notoriously overconfident. Velarix implements **Confidence Decay**:
*   Root observations (Direct from the LLM) are capped at `0.75`.
*   Derived conclusions (Supported by 3+ independent sources/justifications) can climb to `0.95`.
*   This prevents "Circular Certainty" from locking the agent into a hallucinated plan.

## 3. Counterfactual Reasoning
When the `explain_reasoning` tool is called, Velarix doesn't just look at what happened. It performs a **Graph Retraction Simulation**. It hypothetically removes the fact in question and re-runs the propagation. The difference between the "Actual State" and the "Simulated State" is returned as the causal explanation.

## 4. The Sandwich Pattern
Velarix integrates with model providers (like OpenAI) using the Sandwich Pattern:
1.  **Inject:** Fetches the current "Ground Truth" from the Go server and injects it into the system prompt.
2.  **Instrument:** Auto-injects the `record_observation` and `explain_reasoning` tools.
3.  **Inspect:** The model output is scanned; any new tool calls are automatically asserted back to the Go engine as new facts or justifications.

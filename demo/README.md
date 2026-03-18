# Velarix Killer Demo

This directory contains the proof-of-concept for why Velarix is a mandatory layer for AI Agents.

## The Problem
When an AI agent's prompt history (managed by standard vector databases) fills up with assumptions based on a premise that is later proven false, the agent "hallucinates" and mixes old logic with new requirements.

## The Demo
This script (`agent_pivot.py`) simulates an AI agent building a React application.
1. The agent observes the project uses React 17.
2. It builds a chain of thought based on React 17 (e.g., using `ReactDOM.render`).
3. The user interrupts and says "Use React 18".
4. We invalidate the root premise.
5. **The Magic:** Velarix instantly collapses the *entire* dependent chain of thought in $O(1)$ time. The agent's memory is perfectly clean to pivot to React 18.

## How to run

1. Start the Velarix Go Server from the project root:
```bash
go run main.go
```

2. Open a new terminal, install the python SDK requirements, and run the demo:
```bash
# From the project root
pip install -e ./sdks/python
python demo/agent_pivot.py
```

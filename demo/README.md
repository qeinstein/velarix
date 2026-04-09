# Demo

Velarix ships with one canonical product demo and two framework integration demos.

## Canonical Demo

`approval_integrity.py` is the canonical product demo.

It covers the full product loop:

- approval facts are recorded
- a decision is created
- an upstream fact changes
- the decision becomes stale
- execution is blocked
- the API explains why

### Run The Canonical Demo

Start the API from the project root:

```bash
export VELARIX_ENV=dev
export VELARIX_API_KEY=dev-admin-key
export VELARIX_BADGER_PATH="$(mktemp -d)"
go run main.go
```

In a second terminal:

```bash
pip install -e ./sdks/python
export VELARIX_API_KEY=dev-admin-key
python demo/approval_integrity.py
```

## Framework Demos

### LangGraph

`langgraph_integration.py` demonstrates `VelarixLangGraphMemory` as a LangGraph checkpoint layer backed by Velarix session history.

Install requirements:

```bash
pip install -e './sdks/python[langgraph]'
```

### CrewAI

`crewai_integration.py` demonstrates `VelarixCrewAIMemory` injecting a query-aware belief slice into a CrewAI task and persisting the resulting output back into Velarix.

Install requirements:

```bash
pip install -e './sdks/python[crewai]' crewai
```

## Product Position

`approval_integrity.py` is the canonical product demo.

The LangGraph and CrewAI examples show supported integration surfaces around the core execution-integrity flow.

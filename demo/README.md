# Demo

Velarix ships with one canonical product demo, four framework integration demos, and one deprecated compatibility wrapper.

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
export VELARIX_BADGER_PATH="$(mktemp -d)"
go run main.go --lite
```

In a second terminal:

```bash
pip install -e ./sdks/python
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

### LangChain

`langchain_integration.py` demonstrates `VelarixLangChainChatModel`, which wraps a LangChain chat model and injects the Velarix runtime protocol plus tool handling.

### LlamaIndex

`llamaindex_integration.py` demonstrates `VelarixRetriever`, which turns a Velarix belief slice into LlamaIndex retrieval results.

### Deprecated wrapper

`agent_pivot.py` is a compatibility wrapper that simply runs `approval_integrity.py`.

## Product Position

`approval_integrity.py` is the canonical product demo.

The LangGraph and CrewAI examples show supported integration surfaces around the core execution-integrity flow.

# Demo

`approval_integrity.py` is the canonical demo for this repository.

It shows a simple internal approval workflow:
- root facts establish the state used by an approval recommendation,
- a derived fact represents the recommendation,
- a history entry records the recommendation event,
- invalidating one upstream fact collapses the recommendation before action.

## Run The Demo

Start the API from the project root:

```bash
export VELARIX_ENV=dev
export VELARIX_API_KEY=dev-admin-key
go run main.go
```

In a second terminal:

```bash
pip install -e ./sdks/python
export VELARIX_API_KEY=dev-admin-key
python demo/approval_integrity.py
```

## Notes

- `agent_pivot.py` now forwards to the canonical approval-integrity demo for backwards compatibility.
- Other integration examples in this folder are exploratory and are not the maintained demo path for the V1 wedge.

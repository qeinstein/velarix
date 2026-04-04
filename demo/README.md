# Demo

`approval_integrity.py` is the canonical demo for this repository.

It demonstrates the product the repo should be judged on:

- approval facts are recorded
- a decision is created from those facts
- an upstream fact changes
- the decision becomes stale
- execution is blocked
- the API explains why

## Run The Demo

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

## Notes

- `approval_integrity.py` is the maintained demo path
- other integration examples in this directory are exploratory
- the repo should not be pitched around those exploratory examples


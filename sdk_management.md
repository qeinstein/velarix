# SDK Management: Deployment & Testing

Velarix provides native SDKs for Python and TypeScript to facilitate deterministic reasoning in agentic workflows.

## 🐍 Python SDK (`sdks/python`)

### Deployment
The Python SDK uses `setuptools`. To deploy to a registry (like PyPI):
1.  **Build**: `python setup.py sdist bdist_wheel`
2.  **Upload**: `twine upload dist/*`

### Testing
We use `pytest` for unit and integration testing.
- **Run all tests**: `pytest sdks/python/tests`
- **Key Test Suites**:
    - `test_openai_adapter.py`: Validates the one-line swap logic.
    - `test_sdk_integration.py`: Tests raw client connectivity and session management.
    - `test_embed.py`: Verifies embedding and fact extraction logic.

---

## 🟦 TypeScript SDK (`sdks/typescript`)

### Deployment
The TypeScript SDK is managed via `npm`.
1.  **Build**: `npx tsc` (Compiles TS to JS in the `dist` or root directory).
2.  **Publish**: `npm publish`

### Testing
Testing for the TypeScript SDK is currently evolving. 
- **Current State**: Manual verification via local link (`npm link`) or integration with the `console` frontend.
- **Roadmap**: Integration of `vitest` or `jest` for automated unit testing.

---

## 🧪 End-to-End Verification

Before any SDK or Backend release, follow the **E2E Production Checklist** in [END_TO_END_TEST.md](file:///home/fluxx/Workspace/casualdb/END_TO_END_TEST.md).

### Go Core Tests
Run these from the project root to ensure the underlying epistemic engine is stable:
```bash
go test ./tests/...
```

### Manual E2E Checklist Highlights
- [ ] **Tenant Isolation**: Verify Org A cannot access Org B's sessions.
- [ ] **Encryption Enforcement**: Ensure server fails to boot in `prod` without an encryption key.
- [ ] **Causal Collapse**: Validate that invalidating a root fact correctly propagates through the engine.

# Velarix Documentation

Welcome to the Velarix documentation. Velarix is a production-hardened belief-tracking engine designed for AI agents operating in regulated, high-stakes environments. It replaces logically flat memory with a **Stateful Logical Graph** that enforces reasoning integrity.

## 📚 Table of Contents

- [**Architecture**](ARCHITECTURE.md): Deep dive into the Epistemic Kernel, Dominator Trees, and Causal Logic.
- [**API Reference**](API_REFERENCE.md): Detailed documentation for the Velarix REST API (v1).
- [**Security & Compliance**](SECURITY.md): Authentication, Tenant Isolation, Encryption, and SOC2/HIPAA audit trails.
- [**Integration Guide**](INTEGRATION_GUIDE.md): Using Velarix with Python/TypeScript SDKs and LLM frameworks like LangChain and LlamaIndex.
- [**Operations & Maintenance**](OPERATIONS.md): Monitoring, backups, rate limiting, and persistence.
- [**Error Codes & Troubleshooting**](ERRORS.md): Common error scenarios and how to resolve them.

## 🚀 Quick Start

To get Velarix running locally:

1. **Start the Kernel**:
   ```bash
   export VELARIX_ENCRYPTION_KEY="your-32-byte-secure-key-here"
   go run main.go
   ```

2. **Access the API**:
   The API is available at `http://localhost:8080/v1`.

3. **Explore with Swagger**:
   View the OpenAPI specification at `http://localhost:8080/docs/openapi.yaml`.

## 🛠 SDKs

- [**Python SDK**](../sdks/python/README.md)
- [**TypeScript SDK**](../sdks/typescript/README.md)

## 🌐 Control Plane

The Velarix Console provides a visual interface for managing sessions and visualizing causal graphs.
```bash
cd console
npm install && npm run dev
```

---
*Velarix: Building the trust layer for autonomous healthcare.*

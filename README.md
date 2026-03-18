# Velarix: The Epistemic State Layer for AI Agents

Stop manually passing context strings and managing messy agent memory. **Velarix** is a multi-tenant epistemic middleware that acts as a logical firewall for your agents. You visit **velarix.dev/keys** to get an API key, `pip install velarix`, and point the SDK at the hosted URL. The Velarix Orchestrator handles the entire context loop automatically—you never have to run or manage the Go server yourself.

---

## Quickstart in 3 Steps

1.  **Get your Key**: Visit [velarix.dev/keys](https://velarix.dev/keys) and enter your email to receive your unique vx_ access token.
2.  **Install the SDK**:
    ```bash
    pip install velarix
    ```
3.  **Swap Your Import**: Replace your standard OpenAI import with the Velarix adapter.

```python
# from openai import OpenAI
from velarix.adapters.openai import OpenAI

client = OpenAI(
    api_key="your-openai-key",
    velarix_api_key="vx_your_token_here", 
    velarix_session_id="user_123"      # Multi-tenant isolation
)

# Injection and extraction are now automatic
response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "I prefer dark mode."}]
)
```

## How It Works

1.  **Spin Up**: Start the Go server or deploy it to your cloud provider.
2.  **Install SDK**: Add `velarix` to your Python environment.
3.  **Swap Import**: Change your OpenAI import to `velarix.adapters.openai`.
4.  **Initialize Session**: Pass a `velarix_session_id` to the client constructor.

Velarix intercepts your LLM calls, seamlessly prepending the valid "Context Slice" to the system prompt and using automatic tool calls to extract new facts and assertions. If a schema is defined, it validates all inputs, ensuring only structured, consistent data enters the reasoning graph.

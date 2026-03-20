import os
from llama_index.core import QueryEngine
from llama_index.core.retrievers import BaseRetriever
from velarix.integrations.llamaindex import VelarixRetriever
from llama_index.core.query_engine import RetrieverQueryEngine

# 1. Setup Velarix Retriever (The Epistemic Layer)
# This will ONLY pull facts that are logically 'Valid' 
# (confidence > 0.6 and not retracted)
retriever = VelarixRetriever(
    session_id="agent_knowledge_base",
    base_url=os.getenv("VELARIX_BASE_URL", "http://localhost:8080"),
    api_key="vx_your_key_here"
)

# 2. Build standard LlamaIndex Query Engine
# Drop-in: We just pass our VelarixRetriever
query_engine = RetrieverQueryEngine.from_args(retriever)

# 3. Query the logically consistent slice
print("--- Querying Epistemic Layer ---")
response = query_engine.query("What is our current stance on quantum safety?")

print(f"Agent Response: {response}")
print("\n--- Sources (Logically Valid Facts Only) ---")
for node in response.source_nodes:
    print(f"Fact ID: {node.id_} | Score: {node.score}")

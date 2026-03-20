from typing import List, Optional
from llama_index.core.base.base_retriever import BaseRetriever
from llama_index.core.schema import NodeWithScore, TextNode
from velarix.client import VelarixClient

class VelarixRetriever(BaseRetriever):
    """
    A LlamaIndex retriever that queries the Velarix Epistemic State Layer.
    Returns only logically 'Valid' facts as nodes.
    """
    def __init__(
        self, 
        session_id: str,
        client: Optional[VelarixClient] = None,
        base_url: str = "http://localhost:8080",
        api_key: Optional[str] = None,
        max_facts: int = 20
    ):
        super().__init__()
        self.client = client or VelarixClient(base_url=base_url, api_key=api_key)
        self.session_id = session_id
        self.max_facts = max_facts

    def _retrieve(self, query_bundle: Any) -> List[NodeWithScore]:
        """Fetch the valid slice from Velarix and convert to LlamaIndex nodes."""
        session = self.client.session(self.session_id)
        
        # Get the logically valid facts
        facts = session.get_slice(format="json", max_facts=self.max_facts)
        
        nodes = []
        for fact in facts:
            # Convert fact to text node
            payload_str = str(fact.get("payload", {}))
            node = TextNode(
                text=f"Fact ID: {fact['id']}\nContent: {payload_str}",
                id_=fact['id'],
                metadata=fact.get("payload", {})
            )
            # Epistemic retrieval doesn't have a semantic similarity score in the same way, 
            # so we treat all 'Valid' facts as top-tier matches (1.0).
            nodes.append(NodeWithScore(node=node, score=1.0))
            
        return nodes

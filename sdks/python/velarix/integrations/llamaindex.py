from typing import Any, List, Optional
from llama_index.core.base.base_retriever import BaseRetriever
from llama_index.core.schema import NodeWithScore, TextNode
from velarix.client import VelarixClient


class VelarixRetriever(BaseRetriever):
    """LlamaIndex retriever backed by the Velarix Epistemic State Layer.

    Uses semantic search when a query string is available so that returned
    nodes carry real similarity scores. Falls back to a valid-facts slice
    (scored by resolved_status) when no query is provided.
    """

    def __init__(
        self,
        session_id: str,
        client: Optional[VelarixClient] = None,
        base_url: str = "http://localhost:8080",
        api_key: Optional[str] = None,
        max_facts: int = 20,
    ):
        super().__init__()
        self.client = client or VelarixClient(base_url=base_url, api_key=api_key)
        self.session_id = session_id
        self.max_facts = max_facts

    def _retrieve(self, query_bundle: Any) -> List[NodeWithScore]:
        session = self.client.session(self.session_id)
        query = getattr(query_bundle, "query_str", "") or str(query_bundle)

        if query:
            # Use semantic search so nodes get real similarity scores.
            results = session.semantic_search(query, limit=self.max_facts, valid_only=True)
            nodes = []
            for item in results:
                fact = item if isinstance(item, dict) else {}
                score = float(fact.get("score", fact.get("similarity", 1.0)))
                payload_str = str(fact.get("payload", {}))
                node = TextNode(
                    text=f"Fact ID: {fact.get('id', '')}\nContent: {payload_str}",
                    id_=str(fact.get("id", "")),
                    metadata={k: v for k, v in (fact.get("payload") or {}).items() if isinstance(v, str)},
                )
                nodes.append(NodeWithScore(node=node, score=score))
            return nodes

        # No query — fall back to slice, scoring by resolved_status.
        facts = session.get_slice(
            format="json",
            max_facts=self.max_facts,
            strategy="hybrid",
            include_dependencies=True,
        )
        nodes = []
        for fact in facts:
            score = float(fact.get("resolved_status", 1.0))
            payload_str = str(fact.get("payload", {}))
            node = TextNode(
                text=f"Fact ID: {fact.get('id', '')}\nContent: {payload_str}",
                id_=str(fact.get("id", "")),
                metadata={k: v for k, v in (fact.get("payload") or {}).items() if isinstance(v, str)},
            )
            nodes.append(NodeWithScore(node=node, score=score))
        return nodes

from typing import Any, Dict, List, Optional

from velarix.client import VelarixClient, VelarixSession


class VelarixCrewAIMemory:
    """Build CrewAI prompt context from a Velarix session and persist observations back to it."""

    def __init__(
        self,
        session: Optional[VelarixSession] = None,
        *,
        session_id: Optional[str] = None,
        client: Optional[VelarixClient] = None,
        base_url: str = "http://localhost:8080",
        api_key: Optional[str] = None,
        max_facts: int = 80,
        max_chars: int = 12000,
    ) -> None:
        if session is not None:
            self.session = session
        else:
            if not session_id:
                raise ValueError("session_id is required when no session is provided")
            resolved_client = client or VelarixClient(base_url=base_url, api_key=api_key)
            self.session = resolved_client.session(session_id)
        self.max_facts = max_facts
        self.max_chars = max_chars

    def build_context(self, query: Optional[str] = None) -> str:
        """Return a markdown memory slice for the current session."""
        return self.session.get_slice(
            format="markdown",
            max_facts=self.max_facts,
            query=query,
            strategy="hybrid",
            include_dependencies=True,
            max_chars=self.max_chars,
        )

    def augment_description(self, description: str, *, query: Optional[str] = None) -> str:
        """Append a Velarix memory section to an agent or task description."""
        context = self.build_context(query=query)
        return f"{description}\n\n## Velarix Beliefs\n{context}"

    def record_observation(
        self,
        fact_id: str,
        payload: Dict[str, Any],
        *,
        justifications: Optional[List[List[str]]] = None,
        confidence: float = 0.75,
    ) -> Dict[str, Any]:
        """Persist a root observation or derived fact for a CrewAI run."""
        if justifications:
            return self.session.derive(fact_id, justifications, payload)
        return self.session.observe(fact_id, payload, confidence=confidence)

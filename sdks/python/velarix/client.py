import json
import os
import socket
import subprocess
import time
import signal
import asyncio
import uuid
from typing import List, Dict, Any, Optional, Generator, Union, AsyncGenerator, Tuple
import requests
import httpx

class VelarixRuntimeError(Exception):
    """Raised when the Velarix sidecar fails to start or crashes."""
    pass


def _build_slice_params(
    format: str,
    max_facts: int,
    *,
    query: Optional[str] = None,
    strategy: Optional[str] = None,
    include_dependencies: Optional[bool] = None,
    include_invalid: bool = False,
    max_chars: Optional[int] = None,
) -> Dict[str, Any]:
    params: Dict[str, Any] = {"format": format, "max_facts": max_facts}
    if query:
        params["query"] = query
    if strategy:
        params["strategy"] = strategy
    if include_dependencies is not None:
        params["include_dependencies"] = str(bool(include_dependencies)).lower()
    if include_invalid:
        params["include_invalid"] = "true"
    if max_chars is not None:
        params["max_chars"] = int(max_chars)
    return params


def _slice_cache_key(
    format: str,
    max_facts: int,
    *,
    query: Optional[str] = None,
    strategy: Optional[str] = None,
    include_dependencies: Optional[bool] = None,
    include_invalid: bool = False,
    max_chars: Optional[int] = None,
) -> Tuple[Any, ...]:
    return (
        format,
        int(max_facts),
        (query or "").strip(),
        (strategy or "").strip().lower(),
        include_dependencies,
        bool(include_invalid),
        int(max_chars or 0),
    )

class SidecarManager:
    """Manages the lifecycle of the Go-based Velarix sidecar process."""
    def __init__(self, binary_path: Optional[str] = None, data_dir: Optional[str] = None):
        self.binary_path = binary_path or "velarix"
        self.data_dir = data_dir or "velarix.data"
        self.process: Optional[subprocess.Popen] = None
        self.port: Optional[int] = None
        self.url: Optional[str] = None

    def _find_free_port(self) -> int:
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
            s.bind(('', 0))
            return s.getsockname()[1]

    def _is_binary_available(self) -> bool:
        try:
            subprocess.run([self.binary_path, "--version"], capture_output=True)
            return True
        except (FileNotFoundError, PermissionError):
            return False

    def start(self):
        """Start the local Velarix sidecar and wait for `/health`."""
        if not self._is_binary_available():
            raise VelarixRuntimeError(
                "Velarix binary not found. Run 'pip install velarix[local]' or "
                "download the binary from https://velarix.dev/install and add it to your PATH."
            )

        self.port = self._find_free_port()
        self.url = f"http://localhost:{self.port}"
        
        env = os.environ.copy()
        env["PORT"] = str(self.port)
        
        try:
            self.process = subprocess.Popen(
                [self.binary_path],
                env=env,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True
            )
        except Exception as e:
            raise VelarixRuntimeError(f"Failed to start Velarix sidecar: {e}")

        # Wait for health check
        retries = 30
        while retries > 0:
            if self.process.poll() is not None:
                _, stderr = self.process.communicate()
                raise VelarixRuntimeError(f"Velarix sidecar crashed on startup: {stderr}")
            try:
                resp = requests.get(f"{self.url}/health", timeout=1)
                if resp.status_code == 200:
                    return
            except requests.RequestException:
                pass
            time.sleep(0.2)
            retries -= 1
        
        self.stop()
        raise VelarixRuntimeError("Timed out waiting for Velarix sidecar to become healthy.")

    def stop(self):
        """Stop the sidecar process if it is running."""
        if self.process:
            self.process.terminate()
            try:
                self.process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self.process.kill()
            self.process = None

class VelarixSession:
    """A context-bound session for interacting with Velarix."""
    def __init__(self, client: 'VelarixClient', session_id: str):
        self.client = client
        self.session_id = session_id
        self.base_url = f"{client.base_url}/v1/s/{session_id}"
        self._slice_cache: Dict[Tuple[str, int], Tuple[float, Any]] = {}

    def _headers(self):
        return self.client.headers

    def _idem_headers(self, idempotency_key: Optional[str] = None) -> Dict[str, str]:
        h = dict(self._headers() or {})
        h["Idempotency-Key"] = idempotency_key or f"idem_{uuid.uuid4().hex}"
        return h

    def _clear_cache(self):
        self._slice_cache.clear()

    def observe(
        self,
        fact_id: str,
        payload: Optional[Dict[str, Any]] = None,
        idempotency_key: Optional[str] = None,
        confidence: float = 1.0,
    ) -> Dict[str, Any]:
        """Assert a root fact into the session."""
        self._clear_cache()
        data = {"id": fact_id, "is_root": True, "manual_status": float(confidence), "payload": payload or {}}
        resp = self.client._request("POST", f"{self.base_url}/facts", json=data, headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    def derive(self, fact_id: str, justifications: List[List[str]], payload: Optional[Dict[str, Any]] = None, idempotency_key: Optional[str] = None) -> Dict[str, Any]:
        """Assert a derived fact with OR-of-AND justifications."""
        self._clear_cache()
        data = {"id": fact_id, "is_root": False, "justification_sets": justifications, "payload": payload or {}}
        resp = self.client._request("POST", f"{self.base_url}/facts", json=data, headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    def get_slice(
        self,
        format: str = "json",
        max_facts: int = 50,
        *,
        query: Optional[str] = None,
        strategy: Optional[str] = None,
        include_dependencies: Optional[bool] = None,
        include_invalid: bool = False,
        max_chars: Optional[int] = None,
    ) -> Union[List[Dict[str, Any]], str]:
        """Fetch a ranked session slice as JSON or markdown."""
        # Cache Check
        cache_key = _slice_cache_key(
            format,
            max_facts,
            query=query,
            strategy=strategy,
            include_dependencies=include_dependencies,
            include_invalid=include_invalid,
            max_chars=max_chars,
        )
        if self.client.cache_ttl > 0:
            if cache_key in self._slice_cache:
                timestamp, data = self._slice_cache[cache_key]
                if time.time() - timestamp < self.client.cache_ttl:
                    return data

        resp = self.client._request(
            "GET",
            f"{self.base_url}/slice",
            params=_build_slice_params(
                format,
                max_facts,
                query=query,
                strategy=strategy,
                include_dependencies=include_dependencies,
                include_invalid=include_invalid,
                max_chars=max_chars,
            ),
            headers=self._headers(),
        )
        resp.raise_for_status()
        data = resp.text if format == "markdown" else resp.json()
        
        # Cache Update
        if self.client.cache_ttl > 0:
            self._slice_cache[cache_key] = (time.time(), data)
            
        return data

    def set_config(self, schema: Optional[str] = None, mode: Optional[str] = None, idempotency_key: Optional[str] = None) -> Dict[str, Any]:
        """Update the session schema or enforcement mode."""
        self._clear_cache()
        data = {}
        if schema is not None: data["schema"] = schema
        if mode is not None: data["enforcement_mode"] = mode
        resp = self.client._request("POST", f"{self.base_url}/config", json=data, headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    def get_fact(self, fact_id: str) -> Dict[str, Any]:
        """Fetch one fact by ID."""
        resp = self.client._request("GET", f"{self.base_url}/facts/{fact_id}", headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    def verify_fact(
        self,
        fact_id: str,
        status: str,
        *,
        method: str = "",
        source_ref: str = "",
        reason: str = "",
        verified_at: Optional[int] = None,
        idempotency_key: Optional[str] = None,
    ) -> Dict[str, Any]:
        """
        Admin-only: update verification metadata for a fact.

        Args:
            fact_id: Fact ID in the session namespace.
            status: One of "unverified", "verified", "rejected".
            method: Optional verification method label (e.g. "tool", "human", "db").
            source_ref: Optional external reference (e.g. URL, ticket id, database key).
            reason: Optional human-readable note.
            verified_at: Optional unix ms timestamp; defaults to now server-side.
        """
        self._clear_cache()
        body: Dict[str, Any] = {"status": status}
        if method:
            body["method"] = method
        if source_ref:
            body["source_ref"] = source_ref
        if reason:
            body["reason"] = reason
        if verified_at is not None:
            body["verified_at"] = int(verified_at)
        resp = self.client._request(
            "POST",
            f"{self.base_url}/facts/{fact_id}/verify",
            json=body,
            headers=self._idem_headers(idempotency_key),
        )
        resp.raise_for_status()
        return resp.json()

    def get_history(self) -> List[Dict[str, Any]]:
        """Return the persisted journal for the session."""
        resp = self.client._request("GET", f"{self.base_url}/history", headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    def append_history(
        self,
        event_type: str,
        payload: Optional[Dict[str, Any]] = None,
        fact_id: Optional[str] = None,
        idempotency_key: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Append a journal entry on deployments that expose the history-write route."""
        if not event_type:
            raise ValueError("event_type is required")
        body: Dict[str, Any] = {"type": event_type}
        if payload is not None:
            body["payload"] = payload
        if fact_id:
            body["fact_id"] = fact_id
        resp = self.client._request("POST", f"{self.base_url}/history", json=body, headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    def explain(
        self,
        fact_id: Optional[str] = None,
        timestamp: Optional[str] = None,
        counterfactual_fact_id: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Fetch a structured explanation for a fact or point in time."""
        params = {}
        if fact_id:
            params["fact_id"] = fact_id
        if timestamp:
            params["timestamp"] = timestamp
        if counterfactual_fact_id:
            params["counterfactual_fact_id"] = counterfactual_fact_id
        resp = self.client._request("GET", f"{self.base_url}/explain", params=params, headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    def revalidate(self, idempotency_key: Optional[str] = None) -> Dict[str, Any]:
        """Replay session history and rebuild the current in-memory state."""
        self._clear_cache()
        resp = self.client._request("POST", f"{self.base_url}/revalidate", headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    def extract_and_assert(
        self,
        llm_output: str,
        session_context: str = "",
        auto_retract_contradictions: bool = False,
    ) -> Dict[str, Any]:
        """Run extract-and-assert against an OpenAI-compatible backend."""
        body = {
            "llm_output": llm_output,
            "session_context": session_context,
            "auto_retract_contradictions": auto_retract_contradictions,
        }
        resp = self.client._request(
            "POST",
            f"{self.base_url}/extract-and-assert",
            json=body,
            headers=self._headers(),
        )
        resp.raise_for_status()
        self._clear_cache()
        return resp.json()

    def record_perception(
        self,
        fact_id: str,
        payload: Optional[Dict[str, Any]] = None,
        *,
        confidence: float = 0.75,
        modality: Optional[str] = None,
        provider: Optional[str] = None,
        model: Optional[str] = None,
        embedding: Optional[List[float]] = None,
        metadata: Optional[Dict[str, Any]] = None,
        idempotency_key: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Persist a perceptual or model-derived root fact."""
        self._clear_cache()
        body: Dict[str, Any] = {
            "id": fact_id,
            "payload": payload or {},
            "confidence": float(confidence),
        }
        if modality:
            body["modality"] = modality
        if provider:
            body["provider"] = provider
        if model:
            body["model"] = model
        if embedding is not None:
            body["embedding"] = embedding
        if metadata:
            body["metadata"] = metadata
        resp = self.client._request("POST", f"{self.base_url}/percepts", json=body, headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    def invalidate(
        self,
        fact_id: str,
        idempotency_key: Optional[str] = None,
        *,
        reason: str = "",
        force: bool = False,
    ) -> Dict[str, Any]:
        """Invalidate a root fact."""
        self._clear_cache()
        body: Dict[str, Any] = {}
        if reason:
            body["reason"] = reason
        if force:
            body["force"] = True
        kwargs: Dict[str, Any] = {"headers": self._idem_headers(idempotency_key)}
        if body:
            kwargs["json"] = body
        resp = self.client._request("POST", f"{self.base_url}/facts/{fact_id}/invalidate", **kwargs)
        resp.raise_for_status()
        return resp.json()

    def retract(
        self,
        fact_id: str,
        reason: str = "",
        idempotency_key: Optional[str] = None,
        *,
        force: bool = False,
    ) -> Dict[str, Any]:
        """Retract a fact."""
        self._clear_cache()
        body: Dict[str, Any] = {}
        if reason:
            body["reason"] = reason
        if force:
            body["force"] = True
        resp = self.client._request("POST", f"{self.base_url}/facts/{fact_id}/retract", json=body, headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    def review_fact(
        self,
        fact_id: str,
        status: str,
        *,
        reason: str = "",
        idempotency_key: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Set a fact review status."""
        self._clear_cache()
        body = {"status": status}
        if reason:
            body["reason"] = reason
        resp = self.client._request(
            "POST",
            f"{self.base_url}/facts/{fact_id}/review",
            json=body,
            headers=self._idem_headers(idempotency_key),
        )
        resp.raise_for_status()
        return resp.json()

    def semantic_search(self, query: str, *, limit: int = 10, valid_only: bool = True) -> List[Dict[str, Any]]:
        """Run semantic search over session facts."""
        params = {"q": query, "limit": limit, "valid_only": str(valid_only).lower()}
        resp = self.client._request("GET", f"{self.base_url}/semantic-search", params=params, headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    def consistency_check(
        self,
        *,
        fact_ids: Optional[List[str]] = None,
        max_facts: Optional[int] = None,
        include_invalid: bool = False,
    ) -> Dict[str, Any]:
        """Run the session consistency checker."""
        body: Dict[str, Any] = {"include_invalid": include_invalid}
        if fact_ids:
            body["fact_ids"] = fact_ids
        if max_facts is not None:
            body["max_facts"] = max_facts
        resp = self.client._request("POST", f"{self.base_url}/consistency-check", json=body, headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    def record_reasoning_chain(self, chain: Dict[str, Any]) -> Dict[str, Any]:
        """Persist a reasoning chain."""
        resp = self.client._request("POST", f"{self.base_url}/reasoning-chains", json=chain, headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    def list_reasoning_chains(self) -> List[Dict[str, Any]]:
        """List stored reasoning chains for the session."""
        resp = self.client._request("GET", f"{self.base_url}/reasoning-chains", headers=self._headers())
        resp.raise_for_status()
        return resp.json().get("items", [])

    def verify_reasoning_chain(self, chain_id: str, *, auto_retract: bool = False) -> Dict[str, Any]:
        """Verify a stored reasoning chain."""
        resp = self.client._request(
            "POST",
            f"{self.base_url}/reasoning-chains/{chain_id}/verify",
            json={"auto_retract": auto_retract},
            headers=self._headers(),
        )
        resp.raise_for_status()
        return resp.json()

    def create_decision(
        self,
        decision_type: str,
        *,
        subject_ref: str = "",
        target_ref: str = "",
        fact_id: Optional[str] = None,
        decision_id: Optional[str] = None,
        recommended_action: Optional[str] = None,
        policy_version: Optional[str] = None,
        explanation_summary: Optional[str] = None,
        dependency_fact_ids: Optional[List[str]] = None,
        metadata: Optional[Dict[str, Any]] = None,
        idempotency_key: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Create a first-class decision record."""
        if not decision_type:
            raise ValueError("decision_type is required")
        body: Dict[str, Any] = {
            "decision_type": decision_type,
            "subject_ref": subject_ref,
            "target_ref": target_ref,
        }
        if fact_id:
            body["fact_id"] = fact_id
        if decision_id:
            body["decision_id"] = decision_id
        if recommended_action:
            body["recommended_action"] = recommended_action
        if policy_version:
            body["policy_version"] = policy_version
        if explanation_summary:
            body["explanation_summary"] = explanation_summary
        if dependency_fact_ids:
            body["dependency_fact_ids"] = dependency_fact_ids
        if metadata:
            body["metadata"] = metadata
        resp = self.client._request("POST", f"{self.base_url}/decisions", json=body, headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    def list_decisions(
        self,
        *,
        status: Optional[str] = None,
        subject_ref: Optional[str] = None,
        from_ms: Optional[int] = None,
        to_ms: Optional[int] = None,
        limit: int = 50,
    ) -> List[Dict[str, Any]]:
        """List session decisions with optional filters."""
        params: Dict[str, Any] = {"limit": limit}
        if status:
            params["status"] = status
        if subject_ref:
            params["subject"] = subject_ref
        if from_ms is not None:
            params["from"] = from_ms
        if to_ms is not None:
            params["to"] = to_ms
        resp = self.client._request("GET", f"{self.base_url}/decisions", params=params, headers=self._headers())
        resp.raise_for_status()
        return resp.json().get("items", [])

    def get_decision(self, decision_id: str) -> Dict[str, Any]:
        """Fetch one decision."""
        resp = self.client._request("GET", f"{self.base_url}/decisions/{decision_id}", headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    def recompute_decision(
        self,
        decision_id: str,
        *,
        fact_id: Optional[str] = None,
        dependency_fact_ids: Optional[List[str]] = None,
        idempotency_key: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Recompute decision dependencies and status."""
        body: Dict[str, Any] = {}
        if fact_id:
            body["fact_id"] = fact_id
        if dependency_fact_ids:
            body["dependency_fact_ids"] = dependency_fact_ids
        resp = self.client._request(
            "POST",
            f"{self.base_url}/decisions/{decision_id}/recompute",
            json=body,
            headers=self._idem_headers(idempotency_key),
        )
        resp.raise_for_status()
        return resp.json()

    def execute_check(self, decision_id: str, idempotency_key: Optional[str] = None) -> Dict[str, Any]:
        """Run a fresh execute-check for a decision."""
        resp = self.client._request(
            "POST",
            f"{self.base_url}/decisions/{decision_id}/execute-check",
            headers=self._idem_headers(idempotency_key),
        )
        resp.raise_for_status()
        return resp.json()

    def execute_decision(
        self,
        decision_id: str,
        *,
        execution_ref: Optional[str] = None,
        execution_token: Optional[str] = None,
        idempotency_key: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Execute a decision, fetching a fresh execution token if needed."""
        token = execution_token
        if not token:
            check = self.execute_check(decision_id, idempotency_key=idempotency_key)
            token = check.get("execution_token")
            if not token:
                raise ValueError("execute_check did not return an execution_token; decision is likely blocked")
        body: Dict[str, Any] = {}
        if execution_ref:
            body["execution_ref"] = execution_ref
        body["execution_token"] = token
        resp = self.client._request(
            "POST",
            f"{self.base_url}/decisions/{decision_id}/execute",
            json=body,
            headers=self._idem_headers(idempotency_key),
        )
        resp.raise_for_status()
        return resp.json()

    def get_decision_lineage(self, decision_id: str) -> Dict[str, Any]:
        """Fetch the stored dependency lineage for a decision."""
        resp = self.client._request("GET", f"{self.base_url}/decisions/{decision_id}/lineage", headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    def get_decision_why_blocked(self, decision_id: str) -> Dict[str, Any]:
        """Explain why a decision is blocked."""
        resp = self.client._request("GET", f"{self.base_url}/decisions/{decision_id}/why-blocked", headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    def record_decision(self, kind: str, payload: Optional[Dict[str, Any]] = None, idempotency_key: Optional[str] = None) -> Dict[str, Any]:
        """Append an internal decision-record history entry when the route is exposed."""
        if not kind:
            raise ValueError("kind is required")
        return self.append_history("decision_record", {"kind": kind, **(payload or {})}, idempotency_key=idempotency_key)

class VelarixGlobalFacts:
    """Org-wide global facts shared across all sessions."""

    def __init__(self, client: 'VelarixClient'):
        self.client = client
        self.base_url = f"{client.base_url}/v1/global/facts"

    def assert_fact(
        self,
        fact_id: str,
        payload: Optional[Dict[str, Any]] = None,
        *,
        confidence: float = 1.0,
        metadata: Optional[Dict[str, Any]] = None,
        assertion_kind: Optional[str] = None,
        valid_until: Optional[int] = None,
    ) -> Dict[str, Any]:
        if not fact_id:
            raise ValueError("fact_id is required")
        data: Dict[str, Any] = {
            "id": fact_id,
            "is_root": True,
            "manual_status": float(confidence),
            "payload": payload or {},
        }
        if metadata is not None:
            data["metadata"] = metadata
        if assertion_kind:
            data["assertion_kind"] = assertion_kind
        if valid_until is not None:
            data["valid_until"] = int(valid_until)
        resp = self.client._request("POST", self.base_url, json=data, headers=self.client.headers)
        resp.raise_for_status()
        return resp.json()

    def retract(self, fact_id: str) -> Dict[str, Any]:
        if not fact_id:
            raise ValueError("fact_id is required")
        resp = self.client._request("DELETE", f"{self.base_url}/{fact_id}", headers=self.client.headers)
        resp.raise_for_status()
        return resp.json()

    def list(self) -> List[Dict[str, Any]]:
        resp = self.client._request("GET", self.base_url, headers=self.client.headers)
        resp.raise_for_status()
        data = resp.json()
        if isinstance(data, dict) and "items" in data:
            return data["items"]
        return data


    def delete(self) -> Dict[str, Any]:
        """Archive the session through the org-scoped session endpoint."""
        resp = self.client._request(
            "DELETE",
            f"{self.client.base_url}/v1/org/sessions/{self.session_id}",
            headers=self._headers(),
        )
        resp.raise_for_status()
        self._clear_cache()
        return resp.json()

class VelarixClient:
    """Synchronous Velarix client."""
    def __init__(
        self, 
        base_url: Optional[str] = None, 
        api_key: Optional[str] = None,
        embed_mode: bool = False,
        binary_path: Optional[str] = None,
        cache_ttl: int = 30,
        max_retries: int = 5,
        retry_backoff_base: float = 0.25,
        retry_backoff_max: float = 5.0,
        timeout_s: float = 10.0,
    ):
        self.embed_mode = embed_mode
        self.sidecar: Optional[SidecarManager] = None
        self.cache_ttl = cache_ttl
        
        if embed_mode:
            self.sidecar = SidecarManager(binary_path=binary_path)
            self.sidecar.start()
            self.base_url = self.sidecar.url
        else:
            self.base_url = (base_url or "http://localhost:8080").rstrip("/")
            
        self.api_key = api_key
        self.headers = {"Authorization": f"Bearer {api_key}"} if api_key else {}
        self.max_retries = max(0, int(max_retries))
        self.retry_backoff_base = float(retry_backoff_base)
        self.retry_backoff_max = float(retry_backoff_max)
        self.timeout_s = float(timeout_s)
        self._http = requests.Session()
        self.global_facts = VelarixGlobalFacts(self)

    def _request(self, method: str, url: str, **kwargs) -> requests.Response:
        retryable_status = {429, 502, 503, 504}
        timeout = kwargs.pop("timeout", self.timeout_s)

        for attempt in range(self.max_retries + 1):
            try:
                resp = self._http.request(method, url, timeout=timeout, **kwargs)
            except requests.RequestException:
                if attempt >= self.max_retries:
                    raise
                delay = min(self.retry_backoff_max, self.retry_backoff_base * (2 ** attempt))
                time.sleep(delay)
                continue

            if resp.status_code not in retryable_status or attempt >= self.max_retries:
                return resp

            ra = resp.headers.get("Retry-After")
            if ra:
                try:
                    delay = float(ra)
                except ValueError:
                    delay = min(self.retry_backoff_max, self.retry_backoff_base * (2 ** attempt))
            else:
                delay = min(self.retry_backoff_max, self.retry_backoff_base * (2 ** attempt))
            time.sleep(delay)

        return resp  # pragma: no cover

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        if self.sidecar:
            self.sidecar.stop()

    def __del__(self):
        if hasattr(self, 'sidecar') and self.sidecar:
            self.sidecar.stop()

    def session(self, session_id: str) -> VelarixSession:
        """Bind the client to an existing or future session ID."""
        return VelarixSession(self, session_id)

    def create_session(self, session_id: Optional[str] = None) -> VelarixSession:
        """Create a session handle and initialize it via `set_config()`."""
        resolved_session_id = session_id or str(uuid.uuid4())
        session = self.session(resolved_session_id)
        session.set_config()
        return session

    def get_sessions(self) -> List[Dict[str, Any]]:
        """List org sessions visible to the caller."""
        resp = self._request("GET", f"{self.base_url}/v1/sessions", headers=self.headers)
        resp.raise_for_status()
        return resp.json()

    def get_usage(self) -> Dict[str, Any]:
        """Fetch org usage counters."""
        resp = self._request("GET", f"{self.base_url}/v1/org/usage", headers=self.headers)
        resp.raise_for_status()
        return resp.json()

    def list_org_decisions(
        self,
        *,
        status: Optional[str] = None,
        subject_ref: Optional[str] = None,
        from_ms: Optional[int] = None,
        to_ms: Optional[int] = None,
        limit: int = 50,
    ) -> List[Dict[str, Any]]:
        """List organization decisions with optional filters."""
        params: Dict[str, Any] = {"limit": limit}
        if status:
            params["status"] = status
        if subject_ref:
            params["subject"] = subject_ref
        if from_ms is not None:
            params["from"] = from_ms
        if to_ms is not None:
            params["to"] = to_ms
        resp = self._request("GET", f"{self.base_url}/v1/org/decisions", params=params, headers=self.headers)
        resp.raise_for_status()
        return resp.json().get("items", [])

class AsyncVelarixSession:
    """An asynchronous context-bound session for interacting with Velarix."""
    def __init__(self, client: 'AsyncVelarixClient', session_id: str):
        self.client = client
        self.session_id = session_id
        self.base_url = f"{client.base_url}/v1/s/{session_id}"
        self._slice_cache: Dict[Tuple[str, int], Tuple[float, Any]] = {}

    def _headers(self):
        return self.client.headers

    def _idem_headers(self, idempotency_key: Optional[str] = None) -> Dict[str, str]:
        h = dict(self._headers() or {})
        h["Idempotency-Key"] = idempotency_key or f"idem_{uuid.uuid4().hex}"
        return h

    def _clear_cache(self):
        self._slice_cache.clear()

    async def observe(
        self,
        fact_id: str,
        payload: Optional[Dict[str, Any]] = None,
        idempotency_key: Optional[str] = None,
        confidence: float = 1.0,
    ) -> Dict[str, Any]:
        """Assert a root fact into the session."""
        self._clear_cache()
        data = {"id": fact_id, "is_root": True, "manual_status": float(confidence), "payload": payload or {}}
        resp = await self.client._request("POST", f"{self.base_url}/facts", json=data, headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    async def derive(self, fact_id: str, justifications: List[List[str]], payload: Optional[Dict[str, Any]] = None, idempotency_key: Optional[str] = None) -> Dict[str, Any]:
        """Assert a derived fact with OR-of-AND justifications."""
        self._clear_cache()
        data = {"id": fact_id, "is_root": False, "justification_sets": justifications, "payload": payload or {}}
        resp = await self.client._request("POST", f"{self.base_url}/facts", json=data, headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    async def get_slice(
        self,
        format: str = "json",
        max_facts: int = 50,
        *,
        query: Optional[str] = None,
        strategy: Optional[str] = None,
        include_dependencies: Optional[bool] = None,
        include_invalid: bool = False,
        max_chars: Optional[int] = None,
    ) -> Union[List[Dict[str, Any]], str]:
        """Fetch a ranked session slice as JSON or markdown."""
        # Cache Check
        cache_key = _slice_cache_key(
            format,
            max_facts,
            query=query,
            strategy=strategy,
            include_dependencies=include_dependencies,
            include_invalid=include_invalid,
            max_chars=max_chars,
        )
        if self.client.cache_ttl > 0:
            if cache_key in self._slice_cache:
                timestamp, data = self._slice_cache[cache_key]
                if time.time() - timestamp < self.client.cache_ttl:
                    return data

        resp = await self.client._request(
            "GET",
            f"{self.base_url}/slice",
            params=_build_slice_params(
                format,
                max_facts,
                query=query,
                strategy=strategy,
                include_dependencies=include_dependencies,
                include_invalid=include_invalid,
                max_chars=max_chars,
            ),
            headers=self._headers(),
        )
        resp.raise_for_status()
        data = resp.text if format == "markdown" else resp.json()
        
        # Cache Update
        if self.client.cache_ttl > 0:
            self._slice_cache[cache_key] = (time.time(), data)
            
        return data

    async def set_config(self, schema: Optional[str] = None, mode: Optional[str] = None) -> Dict[str, Any]:
        """Update the session schema or enforcement mode."""
        self._clear_cache()
        data = {}
        if schema is not None: data["schema"] = schema
        if mode is not None: data["enforcement_mode"] = mode
        resp = await self.client._request("POST", f"{self.base_url}/config", json=data, headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    async def get_fact(self, fact_id: str) -> Dict[str, Any]:
        """Fetch one fact by ID."""
        resp = await self.client._request("GET", f"{self.base_url}/facts/{fact_id}", headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    async def verify_fact(
        self,
        fact_id: str,
        status: str,
        *,
        method: str = "",
        source_ref: str = "",
        reason: str = "",
        verified_at: Optional[int] = None,
        idempotency_key: Optional[str] = None,
    ) -> Dict[str, Any]:
        """
        Admin-only: update verification metadata for a fact.

        Args:
            fact_id: Fact ID in the session namespace.
            status: One of "unverified", "verified", "rejected".
            method: Optional verification method label (e.g. "tool", "human", "db").
            source_ref: Optional external reference (e.g. URL, ticket id, database key).
            reason: Optional human-readable note.
            verified_at: Optional unix ms timestamp; defaults to now server-side.
        """
        self._clear_cache()
        body: Dict[str, Any] = {"status": status}
        if method:
            body["method"] = method
        if source_ref:
            body["source_ref"] = source_ref
        if reason:
            body["reason"] = reason
        if verified_at is not None:
            body["verified_at"] = int(verified_at)
        resp = await self.client._request(
            "POST",
            f"{self.base_url}/facts/{fact_id}/verify",
            json=body,
            headers=self._idem_headers(idempotency_key),
        )
        resp.raise_for_status()
        return resp.json()

    async def get_history(self) -> List[Dict[str, Any]]:
        """Return the persisted journal for the session."""
        resp = await self.client._request("GET", f"{self.base_url}/history", headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    async def append_history(
        self,
        event_type: str,
        payload: Optional[Dict[str, Any]] = None,
        fact_id: Optional[str] = None,
        idempotency_key: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Append a journal entry on deployments that expose the history-write route."""
        if not event_type:
            raise ValueError("event_type is required")
        body: Dict[str, Any] = {"type": event_type}
        if payload is not None:
            body["payload"] = payload
        if fact_id:
            body["fact_id"] = fact_id
        resp = await self.client._request("POST", f"{self.base_url}/history", json=body, headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    async def explain(
        self,
        fact_id: Optional[str] = None,
        timestamp: Optional[str] = None,
        counterfactual_fact_id: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Fetch a structured explanation for a fact or point in time."""
        params = {}
        if fact_id:
            params["fact_id"] = fact_id
        if timestamp:
            params["timestamp"] = timestamp
        if counterfactual_fact_id:
            params["counterfactual_fact_id"] = counterfactual_fact_id
        resp = await self.client._request("GET", f"{self.base_url}/explain", params=params, headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    async def revalidate(self, idempotency_key: Optional[str] = None) -> Dict[str, Any]:
        """Replay session history and rebuild the current in-memory state."""
        self._clear_cache()
        resp = await self.client._request("POST", f"{self.base_url}/revalidate", headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    async def extract_and_assert(
        self,
        llm_output: str,
        session_context: str = "",
        auto_retract_contradictions: bool = False,
    ) -> Dict[str, Any]:
        """Run extract-and-assert against an OpenAI-compatible backend."""
        body = {
            "llm_output": llm_output,
            "session_context": session_context,
            "auto_retract_contradictions": auto_retract_contradictions,
        }
        resp = await self.client._request(
            "POST",
            f"{self.base_url}/extract-and-assert",
            json=body,
            headers=self._headers(),
        )
        resp.raise_for_status()
        self._clear_cache()
        return resp.json()

    async def record_perception(
        self,
        fact_id: str,
        payload: Optional[Dict[str, Any]] = None,
        *,
        confidence: float = 0.75,
        modality: Optional[str] = None,
        provider: Optional[str] = None,
        model: Optional[str] = None,
        embedding: Optional[List[float]] = None,
        metadata: Optional[Dict[str, Any]] = None,
        idempotency_key: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Persist a perceptual or model-derived root fact."""
        self._clear_cache()
        body: Dict[str, Any] = {
            "id": fact_id,
            "payload": payload or {},
            "confidence": float(confidence),
        }
        if modality:
            body["modality"] = modality
        if provider:
            body["provider"] = provider
        if model:
            body["model"] = model
        if embedding is not None:
            body["embedding"] = embedding
        if metadata:
            body["metadata"] = metadata
        resp = await self.client._request("POST", f"{self.base_url}/percepts", json=body, headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    async def invalidate(
        self,
        fact_id: str,
        idempotency_key: Optional[str] = None,
        *,
        reason: str = "",
        force: bool = False,
    ) -> Dict[str, Any]:
        """Invalidate a root fact."""
        self._clear_cache()
        body: Dict[str, Any] = {}
        if reason:
            body["reason"] = reason
        if force:
            body["force"] = True
        kwargs: Dict[str, Any] = {"headers": self._idem_headers(idempotency_key)}
        if body:
            kwargs["json"] = body
        resp = await self.client._request("POST", f"{self.base_url}/facts/{fact_id}/invalidate", **kwargs)
        resp.raise_for_status()
        return resp.json()

    async def retract(
        self,
        fact_id: str,
        reason: str = "",
        idempotency_key: Optional[str] = None,
        *,
        force: bool = False,
    ) -> Dict[str, Any]:
        """Retract a fact."""
        self._clear_cache()
        body: Dict[str, Any] = {}
        if reason:
            body["reason"] = reason
        if force:
            body["force"] = True
        resp = await self.client._request("POST", f"{self.base_url}/facts/{fact_id}/retract", json=body, headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    async def review_fact(
        self,
        fact_id: str,
        status: str,
        *,
        reason: str = "",
        idempotency_key: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Set a fact review status."""
        self._clear_cache()
        body = {"status": status}
        if reason:
            body["reason"] = reason
        resp = await self.client._request(
            "POST",
            f"{self.base_url}/facts/{fact_id}/review",
            json=body,
            headers=self._idem_headers(idempotency_key),
        )
        resp.raise_for_status()
        return resp.json()

    async def semantic_search(self, query: str, *, limit: int = 10, valid_only: bool = True) -> List[Dict[str, Any]]:
        """Run semantic search over session facts."""
        params = {"q": query, "limit": limit, "valid_only": str(valid_only).lower()}
        resp = await self.client._request("GET", f"{self.base_url}/semantic-search", params=params, headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    async def consistency_check(
        self,
        *,
        fact_ids: Optional[List[str]] = None,
        max_facts: Optional[int] = None,
        include_invalid: bool = False,
    ) -> Dict[str, Any]:
        """Run the session consistency checker."""
        body: Dict[str, Any] = {"include_invalid": include_invalid}
        if fact_ids:
            body["fact_ids"] = fact_ids
        if max_facts is not None:
            body["max_facts"] = max_facts
        resp = await self.client._request("POST", f"{self.base_url}/consistency-check", json=body, headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    async def record_reasoning_chain(self, chain: Dict[str, Any]) -> Dict[str, Any]:
        """Persist a reasoning chain."""
        resp = await self.client._request("POST", f"{self.base_url}/reasoning-chains", json=chain, headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    async def list_reasoning_chains(self) -> List[Dict[str, Any]]:
        """List stored reasoning chains for the session."""
        resp = await self.client._request("GET", f"{self.base_url}/reasoning-chains", headers=self._headers())
        resp.raise_for_status()
        return resp.json().get("items", [])

    async def verify_reasoning_chain(self, chain_id: str, *, auto_retract: bool = False) -> Dict[str, Any]:
        """Verify a stored reasoning chain."""
        resp = await self.client._request(
            "POST",
            f"{self.base_url}/reasoning-chains/{chain_id}/verify",
            json={"auto_retract": auto_retract},
            headers=self._headers(),
        )
        resp.raise_for_status()
        return resp.json()

    async def create_decision(
        self,
        decision_type: str,
        *,
        subject_ref: str = "",
        target_ref: str = "",
        fact_id: Optional[str] = None,
        decision_id: Optional[str] = None,
        recommended_action: Optional[str] = None,
        policy_version: Optional[str] = None,
        explanation_summary: Optional[str] = None,
        dependency_fact_ids: Optional[List[str]] = None,
        metadata: Optional[Dict[str, Any]] = None,
        idempotency_key: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Create a first-class decision record."""
        if not decision_type:
            raise ValueError("decision_type is required")
        body: Dict[str, Any] = {
            "decision_type": decision_type,
            "subject_ref": subject_ref,
            "target_ref": target_ref,
        }
        if fact_id:
            body["fact_id"] = fact_id
        if decision_id:
            body["decision_id"] = decision_id
        if recommended_action:
            body["recommended_action"] = recommended_action
        if policy_version:
            body["policy_version"] = policy_version
        if explanation_summary:
            body["explanation_summary"] = explanation_summary
        if dependency_fact_ids:
            body["dependency_fact_ids"] = dependency_fact_ids
        if metadata:
            body["metadata"] = metadata
        resp = await self.client._request("POST", f"{self.base_url}/decisions", json=body, headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    async def list_decisions(
        self,
        *,
        status: Optional[str] = None,
        subject_ref: Optional[str] = None,
        from_ms: Optional[int] = None,
        to_ms: Optional[int] = None,
        limit: int = 50,
    ) -> List[Dict[str, Any]]:
        """List session decisions with optional filters."""
        params: Dict[str, Any] = {"limit": limit}
        if status:
            params["status"] = status
        if subject_ref:
            params["subject"] = subject_ref
        if from_ms is not None:
            params["from"] = from_ms
        if to_ms is not None:
            params["to"] = to_ms
        resp = await self.client._request("GET", f"{self.base_url}/decisions", params=params, headers=self._headers())
        resp.raise_for_status()
        return resp.json().get("items", [])

    async def get_decision(self, decision_id: str) -> Dict[str, Any]:
        """Fetch one decision."""
        resp = await self.client._request("GET", f"{self.base_url}/decisions/{decision_id}", headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    async def recompute_decision(
        self,
        decision_id: str,
        *,
        fact_id: Optional[str] = None,
        dependency_fact_ids: Optional[List[str]] = None,
        idempotency_key: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Recompute decision dependencies and status."""
        body: Dict[str, Any] = {}
        if fact_id:
            body["fact_id"] = fact_id
        if dependency_fact_ids:
            body["dependency_fact_ids"] = dependency_fact_ids
        resp = await self.client._request(
            "POST",
            f"{self.base_url}/decisions/{decision_id}/recompute",
            json=body,
            headers=self._idem_headers(idempotency_key),
        )
        resp.raise_for_status()
        return resp.json()

    async def execute_check(self, decision_id: str, idempotency_key: Optional[str] = None) -> Dict[str, Any]:
        """Run a fresh execute-check for a decision."""
        resp = await self.client._request(
            "POST",
            f"{self.base_url}/decisions/{decision_id}/execute-check",
            headers=self._idem_headers(idempotency_key),
        )
        resp.raise_for_status()
        return resp.json()

    async def execute_decision(
        self,
        decision_id: str,
        *,
        execution_ref: Optional[str] = None,
        execution_token: Optional[str] = None,
        idempotency_key: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Execute a decision, fetching a fresh execution token if needed."""
        token = execution_token
        if not token:
            check = await self.execute_check(decision_id, idempotency_key=idempotency_key)
            token = check.get("execution_token")
            if not token:
                raise ValueError("execute_check did not return an execution_token; decision is likely blocked")
        body: Dict[str, Any] = {}
        if execution_ref:
            body["execution_ref"] = execution_ref
        body["execution_token"] = token
        resp = await self.client._request(
            "POST",
            f"{self.base_url}/decisions/{decision_id}/execute",
            json=body,
            headers=self._idem_headers(idempotency_key),
        )
        resp.raise_for_status()
        return resp.json()

    async def get_decision_lineage(self, decision_id: str) -> Dict[str, Any]:
        """Fetch the stored dependency lineage for a decision."""
        resp = await self.client._request("GET", f"{self.base_url}/decisions/{decision_id}/lineage", headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    async def get_decision_why_blocked(self, decision_id: str) -> Dict[str, Any]:
        """Explain why a decision is blocked."""
        resp = await self.client._request("GET", f"{self.base_url}/decisions/{decision_id}/why-blocked", headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    async def record_decision(self, kind: str, payload: Optional[Dict[str, Any]] = None, idempotency_key: Optional[str] = None) -> Dict[str, Any]:
        """Append an internal decision-record history entry when the route is exposed."""
        if not kind:
            raise ValueError("kind is required")
        return await self.append_history("decision_record", {"kind": kind, **(payload or {})}, idempotency_key=idempotency_key)

class AsyncVelarixGlobalFacts:
    """Org-wide global facts shared across all sessions (async)."""

    def __init__(self, client: 'AsyncVelarixClient'):
        self.client = client
        self.base_url = f"{client.base_url}/v1/global/facts"

    async def assert_fact(
        self,
        fact_id: str,
        payload: Optional[Dict[str, Any]] = None,
        *,
        confidence: float = 1.0,
        metadata: Optional[Dict[str, Any]] = None,
        assertion_kind: Optional[str] = None,
        valid_until: Optional[int] = None,
    ) -> Dict[str, Any]:
        if not fact_id:
            raise ValueError("fact_id is required")
        data: Dict[str, Any] = {
            "id": fact_id,
            "is_root": True,
            "manual_status": float(confidence),
            "payload": payload or {},
        }
        if metadata is not None:
            data["metadata"] = metadata
        if assertion_kind:
            data["assertion_kind"] = assertion_kind
        if valid_until is not None:
            data["valid_until"] = int(valid_until)
        resp = await self.client._request("POST", self.base_url, json=data, headers=self.client.headers)
        resp.raise_for_status()
        return resp.json()

    async def retract(self, fact_id: str) -> Dict[str, Any]:
        if not fact_id:
            raise ValueError("fact_id is required")
        resp = await self.client._request("DELETE", f"{self.base_url}/{fact_id}", headers=self.client.headers)
        resp.raise_for_status()
        return resp.json()

    async def list(self) -> List[Dict[str, Any]]:
        resp = await self.client._request("GET", self.base_url, headers=self.client.headers)
        resp.raise_for_status()
        data = resp.json()
        if isinstance(data, dict) and "items" in data:
            return data["items"]
        return data


    async def delete(self) -> Dict[str, Any]:
        """Archive the session through the org-scoped session endpoint."""
        resp = await self.client._request(
            "DELETE",
            f"{self.client.base_url}/v1/org/sessions/{self.session_id}",
            headers=self._headers(),
        )
        resp.raise_for_status()
        self._clear_cache()
        return resp.json()

class AsyncVelarixClient:
    """An asynchronous client for interacting with Velarix."""
    def __init__(
        self, 
        base_url: Optional[str] = None, 
        api_key: Optional[str] = None,
        embed_mode: bool = False,
        binary_path: Optional[str] = None,
        cache_ttl: int = 30,
        max_retries: int = 5,
        retry_backoff_base: float = 0.25,
        retry_backoff_max: float = 5.0,
        timeout_s: float = 10.0,
    ):
        self.embed_mode = embed_mode
        self.sidecar: Optional[SidecarManager] = None
        self._base_url_arg = base_url
        self.api_key = api_key
        self.binary_path = binary_path
        self.cache_ttl = cache_ttl
        self.headers = {"Authorization": f"Bearer {api_key}"} if api_key else {}
        self.base_url = (base_url or "http://localhost:8080").rstrip("/")
        self.max_retries = max(0, int(max_retries))
        self.retry_backoff_base = float(retry_backoff_base)
        self.retry_backoff_max = float(retry_backoff_max)
        self.timeout_s = float(timeout_s)
        self.http_client = httpx.AsyncClient(timeout=self.timeout_s)
        self.global_facts = AsyncVelarixGlobalFacts(self)

    async def _request(self, method: str, url: str, **kwargs) -> httpx.Response:
        retryable_status = {429, 502, 503, 504}
        for attempt in range(self.max_retries + 1):
            try:
                resp = await self.http_client.request(method, url, **kwargs)
            except httpx.RequestError:
                if attempt >= self.max_retries:
                    raise
                delay = min(self.retry_backoff_max, self.retry_backoff_base * (2 ** attempt))
                await asyncio.sleep(delay)
                continue

            if resp.status_code not in retryable_status or attempt >= self.max_retries:
                return resp

            ra = resp.headers.get("Retry-After")
            if ra:
                try:
                    delay = float(ra)
                except ValueError:
                    delay = min(self.retry_backoff_max, self.retry_backoff_base * (2 ** attempt))
            else:
                delay = min(self.retry_backoff_max, self.retry_backoff_base * (2 ** attempt))
            await asyncio.sleep(delay)

        return resp  # pragma: no cover

    async def __aenter__(self):
        if self.embed_mode:
            self.sidecar = SidecarManager(binary_path=self.binary_path)
            await asyncio.to_thread(self.sidecar.start)
            self.base_url = self.sidecar.url
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        await self.http_client.aclose()
        if self.sidecar:
            await asyncio.to_thread(self.sidecar.stop)

    def session(self, session_id: str) -> AsyncVelarixSession:
        """Bind the client to an existing or future session ID."""
        return AsyncVelarixSession(self, session_id)

    async def create_session(self, session_id: Optional[str] = None) -> AsyncVelarixSession:
        """Create a session handle and initialize it via `set_config()`."""
        resolved_session_id = session_id or str(uuid.uuid4())
        session = self.session(resolved_session_id)
        await session.set_config()
        return session

    async def get_sessions(self) -> List[Dict[str, Any]]:
        """List org sessions visible to the caller."""
        resp = await self._request("GET", f"{self.base_url}/v1/sessions", headers=self.headers)
        resp.raise_for_status()
        return resp.json()

    async def get_usage(self) -> Dict[str, Any]:
        """Fetch org usage counters."""
        resp = await self._request("GET", f"{self.base_url}/v1/org/usage", headers=self.headers)
        resp.raise_for_status()
        return resp.json()

    async def list_org_decisions(
        self,
        *,
        status: Optional[str] = None,
        subject_ref: Optional[str] = None,
        from_ms: Optional[int] = None,
        to_ms: Optional[int] = None,
        limit: int = 50,
    ) -> List[Dict[str, Any]]:
        """List organization decisions with optional filters."""
        params: Dict[str, Any] = {"limit": limit}
        if status:
            params["status"] = status
        if subject_ref:
            params["subject"] = subject_ref
        if from_ms is not None:
            params["from"] = from_ms
        if to_ms is not None:
            params["to"] = to_ms
        resp = await self._request("GET", f"{self.base_url}/v1/org/decisions", params=params, headers=self.headers)
        resp.raise_for_status()
        return resp.json().get("items", [])

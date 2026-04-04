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
        self._clear_cache()
        data = {"id": fact_id, "is_root": True, "manual_status": float(confidence), "payload": payload or {}}
        resp = self.client._request("POST", f"{self.base_url}/facts", json=data, headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    def derive(self, fact_id: str, justifications: List[List[str]], payload: Optional[Dict[str, Any]] = None, idempotency_key: Optional[str] = None) -> Dict[str, Any]:
        self._clear_cache()
        data = {"id": fact_id, "is_root": False, "justification_sets": justifications, "payload": payload or {}}
        resp = self.client._request("POST", f"{self.base_url}/facts", json=data, headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    def invalidate(self, fact_id: str, idempotency_key: Optional[str] = None) -> Dict[str, Any]:
        self._clear_cache()
        resp = self.client._request("POST", f"{self.base_url}/facts/{fact_id}/invalidate", headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    def get_slice(self, format: str = "json", max_facts: int = 50) -> Union[List[Dict[str, Any]], str]:
        # Cache Check
        if self.client.cache_ttl > 0:
            key = (format, max_facts)
            if key in self._slice_cache:
                timestamp, data = self._slice_cache[key]
                if time.time() - timestamp < self.client.cache_ttl:
                    return data

        resp = self.client._request("GET", f"{self.base_url}/slice", params={"format": format, "max_facts": max_facts}, headers=self._headers())
        resp.raise_for_status()
        data = resp.text if format == "markdown" else resp.json()
        
        # Cache Update
        if self.client.cache_ttl > 0:
            self._slice_cache[(format, max_facts)] = (time.time(), data)
            
        return data

    def set_config(self, schema: Optional[str] = None, mode: Optional[str] = None, idempotency_key: Optional[str] = None) -> Dict[str, Any]:
        self._clear_cache()
        data = {}
        if schema is not None: data["schema"] = schema
        if mode is not None: data["enforcement_mode"] = mode
        resp = self.client._request("POST", f"{self.base_url}/config", json=data, headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    def get_fact(self, fact_id: str) -> Dict[str, Any]:
        resp = self.client._request("GET", f"{self.base_url}/facts/{fact_id}", headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    def get_history(self) -> List[Dict[str, Any]]:
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
        self._clear_cache()
        resp = self.client._request("POST", f"{self.base_url}/revalidate", headers=self._idem_headers(idempotency_key))
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
        resp = self.client._request("GET", f"{self.base_url}/decisions/{decision_id}/lineage", headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    def get_decision_why_blocked(self, decision_id: str) -> Dict[str, Any]:
        resp = self.client._request("GET", f"{self.base_url}/decisions/{decision_id}/why-blocked", headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    def record_decision(self, kind: str, payload: Optional[Dict[str, Any]] = None, idempotency_key: Optional[str] = None) -> Dict[str, Any]:
        if not kind:
            raise ValueError("kind is required")
        return self.append_history("decision_record", {"kind": kind, **(payload or {})}, idempotency_key=idempotency_key)

class VelarixClient:
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
        return VelarixSession(self, session_id)

    def get_sessions(self) -> List[Dict[str, Any]]:
        resp = self._request("GET", f"{self.base_url}/v1/sessions", headers=self.headers)
        resp.raise_for_status()
        return resp.json()

    def get_usage(self) -> Dict[str, Any]:
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
        self._clear_cache()
        data = {"id": fact_id, "is_root": True, "manual_status": float(confidence), "payload": payload or {}}
        resp = await self.client._request("POST", f"{self.base_url}/facts", json=data, headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    async def derive(self, fact_id: str, justifications: List[List[str]], payload: Optional[Dict[str, Any]] = None, idempotency_key: Optional[str] = None) -> Dict[str, Any]:
        self._clear_cache()
        data = {"id": fact_id, "is_root": False, "justification_sets": justifications, "payload": payload or {}}
        resp = await self.client._request("POST", f"{self.base_url}/facts", json=data, headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    async def invalidate(self, fact_id: str, idempotency_key: Optional[str] = None) -> Dict[str, Any]:
        self._clear_cache()
        resp = await self.client._request("POST", f"{self.base_url}/facts/{fact_id}/invalidate", headers=self._idem_headers(idempotency_key))
        resp.raise_for_status()
        return resp.json()

    async def get_slice(self, format: str = "json", max_facts: int = 50) -> Union[List[Dict[str, Any]], str]:
        # Cache Check
        if self.client.cache_ttl > 0:
            key = (format, max_facts)
            if key in self._slice_cache:
                timestamp, data = self._slice_cache[key]
                if time.time() - timestamp < self.client.cache_ttl:
                    return data

        resp = await self.client._request("GET", f"{self.base_url}/slice", params={"format": format, "max_facts": max_facts}, headers=self._headers())
        resp.raise_for_status()
        data = resp.text if format == "markdown" else resp.json()
        
        # Cache Update
        if self.client.cache_ttl > 0:
            self._slice_cache[(format, max_facts)] = (time.time(), data)
            
        return data

    async def set_config(self, schema: Optional[str] = None, mode: Optional[str] = None) -> Dict[str, Any]:
        self._clear_cache()
        data = {}
        if schema is not None: data["schema"] = schema
        if mode is not None: data["enforcement_mode"] = mode
        resp = await self.client._request("POST", f"{self.base_url}/config", json=data, headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    async def get_fact(self, fact_id: str) -> Dict[str, Any]:
        resp = await self.client._request("GET", f"{self.base_url}/facts/{fact_id}", headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    async def get_history(self) -> List[Dict[str, Any]]:
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
        self._clear_cache()
        resp = await self.client._request("POST", f"{self.base_url}/revalidate", headers=self._idem_headers(idempotency_key))
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
        resp = await self.client._request("GET", f"{self.base_url}/decisions/{decision_id}/lineage", headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    async def get_decision_why_blocked(self, decision_id: str) -> Dict[str, Any]:
        resp = await self.client._request("GET", f"{self.base_url}/decisions/{decision_id}/why-blocked", headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    async def record_decision(self, kind: str, payload: Optional[Dict[str, Any]] = None, idempotency_key: Optional[str] = None) -> Dict[str, Any]:
        if not kind:
            raise ValueError("kind is required")
        return await self.append_history("decision_record", {"kind": kind, **(payload or {})}, idempotency_key=idempotency_key)

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
        return AsyncVelarixSession(self, session_id)

    async def get_sessions(self) -> List[Dict[str, Any]]:
        resp = await self._request("GET", f"{self.base_url}/v1/sessions", headers=self.headers)
        resp.raise_for_status()
        return resp.json()

    async def get_usage(self) -> Dict[str, Any]:
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

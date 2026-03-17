import json
import os
import socket
import subprocess
import time
import signal
from typing import List, Dict, Any, Optional, Generator, Union
import requests

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
        
        # Start the sidecar
        # Note: We'd ideally pass the port via a flag or env var. 
        # Assuming the Go binary supports PORT env var.
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
        self.base_url = f"{client.base_url}/s/{session_id}"

    def _headers(self):
        return self.client.headers

    def observe(self, fact_id: str, payload: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        data = {"ID": fact_id, "IsRoot": True, "ManualStatus": 1.0, "payload": payload or {}}
        resp = requests.post(f"{self.base_url}/facts", json=data, headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    def derive(self, fact_id: str, justifications: List[List[str]], payload: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        data = {"ID": fact_id, "IsRoot": False, "justification_sets": justifications, "payload": payload or {}}
        resp = requests.post(f"{self.base_url}/facts", json=data, headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    def invalidate(self, fact_id: str) -> Dict[str, Any]:
        resp = requests.post(f"{self.base_url}/facts/{fact_id}/invalidate", headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    def get_slice(self, format: str = "json", max_facts: int = 50) -> Union[List[Dict[str, Any]], str]:
        resp = requests.get(f"{self.base_url}/slice", params={"format": format, "max_facts": max_facts}, headers=self._headers())
        resp.raise_for_status()
        if format == "markdown": return resp.text
        return resp.json()

    def set_config(self, schema: Optional[str] = None, mode: Optional[str] = None) -> Dict[str, Any]:
        data = {}
        if schema is not None: data["schema"] = schema
        if mode is not None: data["enforcement_mode"] = mode
        resp = requests.post(f"{self.base_url}/config", json=data, headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    def get_fact(self, fact_id: str) -> Dict[str, Any]:
        resp = requests.get(f"{self.base_url}/facts/{fact_id}", headers=self._headers())
        resp.raise_for_status()
        return resp.json()

    def get_history(self) -> List[Dict[str, Any]]:
        resp = requests.get(f"{self.base_url}/history", headers=self._headers())
        resp.raise_for_status()
        return resp.json()

class VelarixClient:
    def __init__(
        self, 
        base_url: Optional[str] = None, 
        api_key: Optional[str] = None,
        embed_mode: bool = False,
        binary_path: Optional[str] = None
    ):
        self.embed_mode = embed_mode
        self.sidecar: Optional[SidecarManager] = None
        
        if embed_mode:
            self.sidecar = SidecarManager(binary_path=binary_path)
            self.sidecar.start()
            self.base_url = self.sidecar.url
        else:
            self.base_url = (base_url or "http://localhost:8080").rstrip("/")
            
        self.api_key = api_key
        self.headers = {"Authorization": f"Bearer {api_key}"} if api_key else {}

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
        resp = requests.get(f"{self.base_url}/sessions", headers=self.headers)
        resp.raise_for_status()
        return resp.json()

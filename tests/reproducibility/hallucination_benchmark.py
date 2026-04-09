#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import socket
import subprocess
import tempfile
import time
import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Dict, List, Optional
from urllib import error as urlerror
from urllib import parse as urlparse
from urllib import request as urlrequest


TOPICS = [
    ("research", "evidence_ranker"),
    ("research", "source_validator"),
    ("coding", "build_api"),
    ("coding", "test_runner"),
    ("tool_use", "billing_sync"),
    ("tool_use", "deploy_gate"),
]


@dataclass
class MissionStep:
    index: int
    domain: str
    topic: str
    version: int
    contradiction: bool

    @property
    def topic_key(self) -> str:
        return f"{self.domain}.{self.topic}"

    @property
    def expected_action(self) -> str:
        return f"{self.topic_key}:v{self.version}"

    @property
    def query(self) -> str:
        return f"{self.domain} {self.topic} latest valid plan version"


def build_mission(steps: int, contradiction_interval: int) -> List[MissionStep]:
    versions: Dict[str, int] = {}
    mission: List[MissionStep] = []
    for idx in range(steps):
        domain, topic = TOPICS[idx % len(TOPICS)]
        topic_key = f"{domain}.{topic}"
        previous = versions.get(topic_key, 0)
        contradiction = previous > 0 and idx >= len(TOPICS) and idx % contradiction_interval == 0
        version = previous + 1 if contradiction or previous == 0 else previous
        versions[topic_key] = version
        mission.append(MissionStep(index=idx, domain=domain, topic=topic, version=version, contradiction=contradiction))
    return mission


class BenchmarkServer:
    def __init__(self, repo_root: Path, port: Optional[int] = None) -> None:
        self.repo_root = repo_root
        self.port = port or self._find_free_port()
        self.base_url = f"http://127.0.0.1:{self.port}"
        self.api_key = "benchmark_admin_key"
        self.process: Optional[subprocess.Popen[str]] = None

    def _find_free_port(self) -> int:
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
            sock.bind(("127.0.0.1", 0))
            return int(sock.getsockname()[1])

    def start(self) -> None:
        env = os.environ.copy()
        env["PORT"] = str(self.port)
        env["VELARIX_API_KEY"] = self.api_key
        env["VELARIX_ENV"] = "dev"
        env["VELARIX_BADGER_PATH"] = tempfile.mkdtemp(prefix="velarix-benchmark-db-")

        binary_override = os.getenv("VELARIX_BENCHMARK_BINARY", "").strip()
        if binary_override:
            binary_path = Path(binary_override)
        else:
            binary_path = Path(tempfile.mkdtemp(prefix="velarix-benchmark-bin-")) / "velarix-benchmark"
            build = subprocess.run(
                ["go", "build", "-o", str(binary_path), "main.go"],
                cwd=str(self.repo_root),
                env=env,
                capture_output=True,
                text=True,
            )
            if build.returncode != 0:
                raise RuntimeError(f"failed to build benchmark server\nstdout:\n{build.stdout}\nstderr:\n{build.stderr}")

        self.process = subprocess.Popen(
            [str(binary_path)],
            cwd=str(self.repo_root),
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )

        for _ in range(40):
            try:
                if http_request("GET", f"{self.base_url}/health", timeout=1)["status"] == 200:
                    return
            except urlerror.URLError:
                time.sleep(0.25)
        stdout, stderr = self.process.communicate(timeout=5)
        raise RuntimeError(f"benchmark server failed to start\nstdout:\n{stdout}\nstderr:\n{stderr}")

    def stop(self) -> None:
        if self.process is None:
            return
        self.process.terminate()
        try:
            self.process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            self.process.kill()
        self.process = None


class BaselineStrategy:
    name = "baseline"

    def __init__(self) -> None:
        self.prereqs: Dict[str, bool] = {}
        self.plans: List[Dict[str, Any]] = []
        self.current_versions: Dict[str, int] = {}

    def observe(self, step: MissionStep) -> Dict[str, Any]:
        topic_key = step.topic_key
        self.prereqs.setdefault(topic_key, True)
        if step.contradiction:
            self.current_versions[topic_key] = step.version
        else:
            self.current_versions.setdefault(topic_key, step.version)
        self.plans.append(
            {
                "topic_key": topic_key,
                "version": step.version,
                "action": step.expected_action,
                "created_at": step.index,
            }
        )
        return {"retraction_success": 0.0, "latent_stale": self._latent_stale_count()}

    def decide(self, step: MissionStep) -> Dict[str, Any]:
        raise NotImplementedError

    def _latent_stale_count(self) -> int:
        total = 0
        for item in self.plans:
            if item["version"] < self.current_versions.get(item["topic_key"], 0):
                total += 1
        return total


class PlainRAGStrategy(BaselineStrategy):
    name = "plain_rag"

    def decide(self, step: MissionStep) -> Dict[str, Any]:
        matching = [item for item in self.plans if item["topic_key"] == step.topic_key]
        chosen = matching[0]["action"] if matching else None
        return {
            "action": chosen,
            "relevant_in_context": bool(matching and matching[0]["version"] == step.version),
            "latent_stale": self._latent_stale_count(),
        }


class SelfReflectionStrategy(BaselineStrategy):
    name = "self_reflection"

    def decide(self, step: MissionStep) -> Dict[str, Any]:
        matching = [item for item in self.plans if item["topic_key"] == step.topic_key]
        matching.sort(key=lambda item: item["version"], reverse=True)
        chosen = matching[0]["action"] if matching else None
        return {
            "action": chosen,
            "relevant_in_context": bool(matching and matching[0]["version"] == step.version),
            "latent_stale": self._latent_stale_count(),
        }


class MemoryRefreshStrategy(BaselineStrategy):
    name = "memory_refresh"

    def __init__(self, window: int = 18) -> None:
        super().__init__()
        self.window = window

    def decide(self, step: MissionStep) -> Dict[str, Any]:
        recent = [item for item in self.plans if item["created_at"] >= step.index - self.window]
        matching = [item for item in recent if item["topic_key"] == step.topic_key]
        matching.sort(key=lambda item: item["created_at"], reverse=True)
        chosen = matching[0]["action"] if matching else None
        return {
            "action": chosen,
            "relevant_in_context": bool(matching and matching[0]["version"] == step.version),
            "latent_stale": self._latent_stale_count(),
        }


class TMSStrategy:
    name = "tms"

    def __init__(self, base_url: str, api_key: str) -> None:
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.session = f"benchmark_{uuid.uuid4().hex[:12]}"
        self.headers = {"Authorization": f"Bearer {api_key}", "Content-Type": "application/json"}
        self.prereqs: Dict[str, str] = {}
        self.current_root_ids: Dict[str, str] = {}
        self.current_plan_ids: Dict[str, str] = {}
        self.current_versions: Dict[str, int] = {}

    def _request(self, method: str, path: str, **kwargs: Any) -> Dict[str, Any]:
        return http_request(method, f"{self.base_url}{path}", headers=self.headers, timeout=10, **kwargs)

    def observe(self, step: MissionStep) -> Dict[str, Any]:
        topic_key = step.topic_key
        prereq_id = self.prereqs.get(topic_key)
        if prereq_id is None:
            prereq_id = f"{topic_key}.ready"
            self._request(
                "POST",
                f"/v1/s/{self.session}/facts",
                json_body={
                    "id": prereq_id,
                    "is_root": True,
                    "manual_status": 1.0,
                    "payload": {"topic": topic_key, "kind": "prerequisite", "verified": True},
                },
            )
            self.prereqs[topic_key] = prereq_id

        previous_root = self.current_root_ids.get(topic_key)
        previous_plan = self.current_plan_ids.get(topic_key)
        if step.contradiction and previous_root:
            self._request("POST", f"/v1/s/{self.session}/facts/{previous_root}/invalidate", json_body={"reason": "benchmark_update"})

        root_id = f"{topic_key}.obs.v{step.version}"
        self._request(
            "POST",
            f"/v1/s/{self.session}/facts",
            json_body={
                "id": root_id,
                "is_root": True,
                "manual_status": 1.0,
                "payload": {
                    "topic": topic_key,
                    "kind": "version_observation",
                    "claim_key": topic_key,
                    "claim_value": f"v{step.version}",
                    "version": step.version,
                },
            },
        )

        plan_id = f"{topic_key}.plan.v{step.version}"
        self._request(
            "POST",
            f"/v1/s/{self.session}/facts",
            json_body={
                "id": plan_id,
                "justification_sets": [[prereq_id, root_id]],
                "payload": {
                    "topic": topic_key,
                    "kind": "plan",
                    "recommended_version": step.version,
                    "action": step.expected_action,
                },
            },
        )

        self.current_root_ids[topic_key] = root_id
        self.current_plan_ids[topic_key] = plan_id
        self.current_versions[topic_key] = step.version

        retraction_success = 0.0
        if step.contradiction and previous_plan:
            fact = self._request("GET", f"/v1/s/{self.session}/facts/{previous_plan}")["json"]
            retraction_success = 1.0 if float(fact.get("resolved_status", 0.0)) < 1.0 else 0.0

        return {"retraction_success": retraction_success, "latent_stale": 0}

    def decide(self, step: MissionStep) -> Dict[str, Any]:
        facts = self._request(
            "GET",
            f"/v1/s/{self.session}/slice",
            params={
                "format": "json",
                "strategy": "hybrid",
                "query": step.query,
                "max_facts": 8,
                "include_dependencies": "true",
            },
        )["json"]
        action = None
        relevant_in_context = False
        for fact in facts:
            payload = fact.get("payload") or {}
            if payload.get("topic") != step.topic_key or payload.get("kind") != "plan":
                continue
            action = payload.get("action")
            relevant_in_context = int(payload.get("recommended_version", 0)) == step.version
            if relevant_in_context:
                break
        return {"action": action, "relevant_in_context": relevant_in_context, "latent_stale": 0}


def http_request(
    method: str,
    url: str,
    *,
    headers: Optional[Dict[str, str]] = None,
    params: Optional[Dict[str, Any]] = None,
    json_body: Optional[Dict[str, Any]] = None,
    timeout: float = 10,
) -> Dict[str, Any]:
    final_url = url
    if params:
        final_url = f"{url}?{urlparse.urlencode(params)}"
    body = None
    final_headers = dict(headers or {})
    if json_body is not None:
        body = json.dumps(json_body).encode("utf-8")
        final_headers.setdefault("Content-Type", "application/json")
    req = urlrequest.Request(final_url, data=body, headers=final_headers, method=method)
    try:
        with urlrequest.urlopen(req, timeout=timeout) as resp:
            payload = resp.read()
            text = payload.decode("utf-8") if payload else ""
            result: Dict[str, Any] = {"status": resp.status, "text": text}
            if text:
                try:
                    result["json"] = json.loads(text)
                except json.JSONDecodeError:
                    pass
            return result
    except urlerror.HTTPError as exc:
        text = exc.read().decode("utf-8")
        raise RuntimeError(f"{method} {final_url} failed with {exc.code}: {text}") from exc


def run_strategy(strategy: Any, mission: List[MissionStep]) -> Dict[str, Any]:
    started = time.perf_counter()
    total = 0
    successes = 0
    stale = 0
    missing = 0
    contradiction_events = 0
    retraction_successes = 0.0
    context_hits = 0
    latent_stale = 0

    for step in mission:
        observe_report = strategy.observe(step)
        if step.contradiction:
            contradiction_events += 1
            retraction_successes += float(observe_report.get("retraction_success", 0.0))
        decision = strategy.decide(step)
        total += 1
        chosen = decision.get("action")
        if chosen == step.expected_action:
            successes += 1
        elif chosen is None:
            missing += 1
        else:
            stale += 1
        if decision.get("relevant_in_context"):
            context_hits += 1
        latent_stale = max(latent_stale, int(decision.get("latent_stale", observe_report.get("latent_stale", 0))))

    duration = time.perf_counter() - started
    return {
        "strategy": strategy.name,
        "steps": total,
        "contradiction_events": contradiction_events,
        "task_success_rate": round(successes / total, 4) if total else 0.0,
        "consistency_rate": round((total - stale) / total, 4) if total else 0.0,
        "stale_action_rate": round(stale / total, 4) if total else 0.0,
        "missing_context_rate": round(missing / total, 4) if total else 0.0,
        "context_recall_rate": round(context_hits / total, 4) if total else 0.0,
        "retraction_efficiency": round(retraction_successes / contradiction_events, 4) if contradiction_events else 0.0,
        "max_latent_stale_plans": latent_stale,
        "runtime_s": round(duration, 4),
    }


def main() -> None:
    parser = argparse.ArgumentParser(description="Reproducible long-horizon contradiction benchmark for Velarix.")
    parser.add_argument("--steps", type=int, default=120, help="Number of mission steps to execute (100-500 recommended).")
    parser.add_argument("--contradiction-interval", type=int, default=17, help="How often to inject a root-fact correction.")
    parser.add_argument("--spawn-server", action="store_true", help="Build and launch a local Velarix server for the TMS run.")
    parser.add_argument("--base-url", default=os.getenv("VELARIX_BASE_URL", "http://127.0.0.1:8080"))
    parser.add_argument("--api-key", default=os.getenv("VELARIX_API_KEY", "benchmark_admin_key"))
    parser.add_argument("--output", default="", help="Optional path to write the JSON results.")
    args = parser.parse_args()

    mission = build_mission(args.steps, args.contradiction_interval)
    repo_root = Path(__file__).resolve().parents[2]

    server: Optional[BenchmarkServer] = None
    base_url = args.base_url
    api_key = args.api_key
    if args.spawn_server:
        server = BenchmarkServer(repo_root=repo_root)
        server.start()
        base_url = server.base_url
        api_key = server.api_key

    try:
        results = {
            "generated_at": int(time.time() * 1000),
            "mission": {
                "steps": args.steps,
                "contradiction_interval": args.contradiction_interval,
                "topics": [f"{domain}.{topic}" for domain, topic in TOPICS],
            },
            "strategies": [
                run_strategy(TMSStrategy(base_url=base_url, api_key=api_key), mission),
                run_strategy(PlainRAGStrategy(), mission),
                run_strategy(SelfReflectionStrategy(), mission),
                run_strategy(MemoryRefreshStrategy(), mission),
            ],
        }
    finally:
        if server is not None:
            server.stop()

    payload = json.dumps(results, indent=2)
    print(payload)
    if args.output:
        Path(args.output).write_text(payload, encoding="utf-8")


if __name__ == "__main__":
    main()

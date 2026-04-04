import pytest
import os
import requests
import time
import subprocess
import asyncio
import socket
from pathlib import Path
import tempfile
from velarix.client import VelarixClient, AsyncVelarixClient

@pytest.fixture(scope="module", autouse=True)
def velarix_server():
    probe = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    try:
        probe.bind(("127.0.0.1", 0))
    except OSError as exc:
        pytest.skip(f"local TCP listen is not permitted in this environment: {exc}")
    finally:
        probe.close()

    # Attempt to start the server in the background for tests
    env = os.environ.copy()
    env["VELARIX_ENCRYPTION_KEY"] = "test_32_byte_secure_key_12345678"
    env["VELARIX_API_KEY"] = "test_key"
    env["VELARIX_ENV"] = "dev"
    env["PORT"] = "8089"
    env["VELARIX_BADGER_PATH"] = tempfile.mkdtemp(prefix="velarix-sdk-py-")
    env["GOCACHE"] = tempfile.mkdtemp(prefix="velarix-go-cache-")
    repo_root = Path(__file__).resolve().parents[3]
    binary_path = Path(tempfile.mkdtemp(prefix="velarix-sdk-bin-")) / "velarix-test-bin"

    build = subprocess.run(
        ["go", "build", "-o", str(binary_path), "main.go"],
        cwd=str(repo_root),
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    if build.returncode != 0:
        raise RuntimeError(f"failed to build test server\nstdout:\n{build.stdout}\nstderr:\n{build.stderr}")
    
    server_process = subprocess.Popen(
        [str(binary_path)],
        cwd=str(repo_root),
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    
    # Wait for server to be ready
    for _ in range(30):
        try:
            if requests.get("http://localhost:8089/health").status_code == 200:
                break
        except requests.RequestException:
            time.sleep(1)
    else:
        stdout, stderr = server_process.communicate(timeout=5)
        raise RuntimeError(f"test server failed to start\nstdout:\n{stdout}\nstderr:\n{stderr}")
            
    yield
    
    server_process.terminate()
    server_process.wait()

def test_sync_client_lifecycle():
    client = VelarixClient(base_url="http://localhost:8089", api_key="test_key")
    session = client.session("test_sdk_sess")
    
    # Actually perform network calls
    session.observe("sdk_root", payload={"val": 1})
    session.derive("sdk_decision_fact", [["sdk_root"]], payload={"summary": "derived approval"})
    decision = session.create_decision(
        "sdk_approval",
        fact_id="sdk_decision_fact",
        subject_ref="invoice-1",
        target_ref="vendor-1",
        dependency_fact_ids=["sdk_root"],
    )
    facts = session.get_slice()
    assert any(f.get("id") == "sdk_root" for f in facts)
    decisions = session.list_decisions()
    assert any(d.get("decision_id") == decision["decision_id"] for d in decisions)
    check = session.execute_check(decision["decision_id"])
    assert check["executable"] is True
    assert isinstance(check.get("execution_token"), str)
    
    session.invalidate("sdk_root")
    facts_after = session.get_slice() or []
    assert not any(f.get("id") == "sdk_root" for f in facts_after)
    blocked = session.execute_check(decision["decision_id"])
    assert blocked["executable"] is False
    why = session.get_decision_why_blocked(decision["decision_id"])
    assert why["decision"]["decision_id"] == decision["decision_id"]

def test_async_client_lifecycle():
    async def run():
        async with AsyncVelarixClient(base_url="http://localhost:8089", api_key="test_key") as client:
            session = client.session("test_sdk_async_sess")
            
            await session.observe("sdk_async_root", payload={"val": 2})
            await session.derive("sdk_async_decision_fact", [["sdk_async_root"]], payload={"summary": "async derived approval"})
            decision = await session.create_decision(
                "sdk_async_approval",
                fact_id="sdk_async_decision_fact",
                subject_ref="invoice-2",
                target_ref="vendor-2",
                dependency_fact_ids=["sdk_async_root"],
            )
            facts = await session.get_slice() or []
            assert any(f.get("id") == "sdk_async_root" for f in facts)
            check = await session.execute_check(decision["decision_id"])
            assert check["executable"] is True
            assert isinstance(check.get("execution_token"), str)
            await session.invalidate("sdk_async_root")
            blocked = await session.execute_check(decision["decision_id"])
            assert blocked["executable"] is False

    asyncio.run(run())

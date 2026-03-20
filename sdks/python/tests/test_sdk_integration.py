import pytest
import os
import requests
import time
import subprocess
from velarix.client import VelarixClient, AsyncVelarixClient

@pytest.fixture(scope="module", autouse=True)
def velarix_server():
    # Attempt to start the server in the background for tests
    env = os.environ.copy()
    env["VELARIX_ENCRYPTION_KEY"] = "test_32_byte_secure_key_12345678"
    env["VELARIX_API_KEY"] = "test_key"
    env["VELARIX_ENV"] = "dev"
    env["PORT"] = "8089"
    
    server_process = subprocess.Popen(
        ["go", "run", "main.go"],
        cwd="../../", # Assuming running from sdks/python
        env=env,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL
    )
    
    # Wait for server to be ready
    for _ in range(10):
        try:
            if requests.get("http://localhost:8089/health").status_code == 200:
                break
        except:
            time.sleep(1)
            
    yield
    
    server_process.terminate()
    server_process.wait()

def test_sync_client_lifecycle():
    client = VelarixClient(base_url="http://localhost:8089", api_key="test_key")
    session = client.session("test_sdk_sess")
    
    # Actually perform network calls
    session.observe("sdk_root", payload={"val": 1})
    facts = session.get_slice()
    assert any(f.get("id") == "sdk_root" for f in facts)
    
    session.invalidate("sdk_root")
    facts_after = session.get_slice()
    assert not any(f.get("id") == "sdk_root" for f in facts_after)

@pytest.mark.asyncio
async def test_async_client_lifecycle():
    async with AsyncVelarixClient(base_url="http://localhost:8089", api_key="test_key") as client:
        session = client.session("test_sdk_async_sess")
        
        await session.observe("sdk_async_root", payload={"val": 2})
        facts = await session.get_slice()
        assert any(f.get("id") == "sdk_async_root" for f in facts)

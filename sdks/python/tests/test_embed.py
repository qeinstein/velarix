import sys
import os
import time
import subprocess

# Add the SDK path to sys.path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__name__), '..')))

from velarix.client import VelarixClient, VelarixRuntimeError

def test_missing_binary():
    """Verify that the specific error message is shown when the binary is missing."""
    try:
        with VelarixClient(embed_mode=True, binary_path="./non_existent_velarix") as client:
            pass
        assert False, "expected error but got none"
    except VelarixRuntimeError as e:
        assert "Velarix binary not found" in str(e) and "pip install velarix[local]" in str(e)

def test_embed_lifecycle():
    """
    Verify that the sidecar starts and stops correctly.
    """
    # Build the binary first to ensure it's available
    print("Building Go binary for embed test...")
    bin_name = "velarix_test_bin.exe" if sys.platform == "win32" else "velarix_test_bin"
    subprocess.run(["go", "build", "-o", bin_name, "../../main.go"], check=True)
    binary_path = os.path.abspath(bin_name)    
    try:
        os.environ["VELARIX_JWT_SECRET"] = "test_secret_for_embed_test_32_bytes_min"
        os.environ["VELARIX_ENV"] = "dev"
        os.environ["VELARIX_LITE"] = "true"
        with VelarixClient(embed_mode=True, binary_path=binary_path) as client:
            session = client.session("test_session")
            # Should not raise any error
            session.observe("test_fact", {"msg": "hello from embed"})
            
            # Verify it's actually running
            slice_data = session.get_slice()
            assert isinstance(slice_data, list)
            
            # Check port assignment
            assert client.sidecar.port is not None
            assert "localhost" in client.base_url
                
            print("Sidecar is running and responding.")
            
        # After exiting the context, the process should be gone
        time.sleep(1)
        assert client.sidecar.process is None
        
    finally:
        if os.path.exists(binary_path):
            os.remove(binary_path)

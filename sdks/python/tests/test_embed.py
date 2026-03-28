import sys
import os
import time
import subprocess
import requests

# Add the SDK path to sys.path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__name__), '..')))

from velarix.client import VelarixClient, VelarixRuntimeError

def test_missing_binary():
    """Verify that the specific error message is shown when the binary is missing."""
    try:
        with VelarixClient(embed_mode=True, binary_path="./non_existent_velarix") as client:
            pass
        print("FAIL: test_missing_binary - expected error but got none")
        return False
    except VelarixRuntimeError as e:
        if "Velarix binary not found" in str(e) and "pip install velarix[local]" in str(e):
            print("PASS: test_missing_binary")
            return True
        else:
            print(f"FAIL: test_missing_binary - unexpected error message: {e}")
            return False

def test_embed_lifecycle():
    """
    Verify that the sidecar starts and stops correctly.
    """
    # Build the binary first to ensure it's available
    print("Building Go binary for embed test...")
    subprocess.run(["go", "build", "-o", "velarix_test_bin", "../../main.go"], check=True)    
    binary_path = os.path.abspath("velarix_test_bin")
    
    try:
        with VelarixClient(embed_mode=True, binary_path=binary_path) as client:
            session = client.session("test_session")
            # Should not raise any error
            session.observe("test_fact", {"msg": "hello from embed"})
            
            # Verify it's actually running
            sessions = client.get_sessions()
            if not isinstance(sessions, list):
                print(f"FAIL: test_embed_lifecycle - get_sessions returned {type(sessions)}")
                return False
            
            # Check port assignment
            if client.sidecar.port is None or "localhost" not in client.base_url:
                print(f"FAIL: test_embed_lifecycle - invalid sidecar URL: {client.base_url}")
                return False
                
            print("Sidecar is running and responding.")
            
        # After exiting the context, the process should be gone
        time.sleep(1)
        if client.sidecar.process is not None:
             print("FAIL: test_embed_lifecycle - process was not cleared")
             return False
        
        print("PASS: test_embed_lifecycle")
        return True
        
    except Exception as e:
        print(f"FAIL: test_embed_lifecycle - caught exception: {e}")
        return False
    finally:
        if os.path.exists(binary_path):
            os.remove(binary_path)

if __name__ == "__main__":
    s1 = test_missing_binary()
    s2 = test_embed_lifecycle()
    if not (s1 and s2):
        sys.exit(1)

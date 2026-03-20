import os
import sys

# Add the SDK to the path
sys.path.append(os.path.join(os.getcwd(), "sdks", "python"))

try:
    from velarix.adapters.openai import OpenAI
    print("Import successful!")
    
    # Check if we can instantiate it
    # We don't need a real API key for just checking __init__
    client = OpenAI(api_key="sk-test", velarix_session_id="test-session")
    print(f"Client initialized. Session ID: {client.velarix_session_id}")
    print(f"Velarix Base URL: {client.velarix_client.base_url}")
    
    # Check if chat property is correctly overridden
    print(f"Chat type: {type(client.chat)}")
    print(f"Completions type: {type(client.chat.completions)}")
    
except Exception as e:
    print(f"Error: {e}")
    import traceback
    traceback.print_exc()

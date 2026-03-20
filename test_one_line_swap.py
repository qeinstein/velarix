import os
import sys
from unittest.mock import MagicMock, patch

# Add the SDK to the path
sys.path.append(os.path.join(os.getcwd(), "sdks", "python"))

# Mocking the base OpenAI class since we might not have the actual library installed or API key
with patch('openai.OpenAI') as MockBaseOpenAI:
    # Set up the mock structure
    mock_instance = MockBaseOpenAI.return_value
    mock_instance.chat.completions.create.return_value = MagicMock()
    
    # Now try to import our adapter
    from velarix.adapters.openai import OpenAI
    
    print("Attempting to initialize Velarix OpenAI adapter...")
    try:
        # Mock VelarixClient as well to avoid network calls
        with patch('velarix.adapters.openai.VelarixClient') as MockVelarix:
            client = OpenAI(api_key="sk-test", velarix_session_id="test-session")
            print("Initialization successful.")
            
            # Check the chat completions access
            print(f"Chat type: {type(client.chat)}")
            print(f"Completions type: {type(client.chat.completions)}")
            
            # This is the critical part: does it call the base completions?
            # We need to mock the session.get_slice as well
            mock_session = MockVelarix.return_value.session.return_value
            mock_session.get_slice.return_value = "# Mock Context"
            
            print("Attempting a dummy create() call...")
            client.chat.completions.create(
                model="gpt-4",
                messages=[{"role": "user", "content": "hello"}]
            )
            print("Create call completed (mocked).")
            
    except Exception as e:
        print(f"Error: {e}")
        import traceback
        traceback.print_exc()

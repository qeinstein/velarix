import sys
import os
import json
from unittest.mock import MagicMock, patch

# Add the SDK path to sys.path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from velarix.adapters.openai import OpenAI
from velarix.client import VelarixSession

def test_openai_interceptor_parallel_calls():
    # Mock Velarix Session
    mock_session = MagicMock(spec=VelarixSession)
    mock_session.base_url = "http://localhost:8080/s/test-session"
    mock_session._headers.return_value = {}
    mock_session.get_slice.return_value = "## Fact: F1\n```json\n{}\n```"
    
    # Mock Velarix Client
    mock_client = MagicMock()
    mock_client.session.return_value = mock_session
    mock_client.headers = {}
    
    with patch('openai.resources.chat.completions.Completions.create') as mock_create:
        with patch('requests.post') as mock_post:
            # 1. Setup Mocked OpenAI Response with 2 parallel tool calls
            mock_response = MagicMock()
            mock_tool_call_1 = MagicMock()
            mock_tool_call_1.function.name = "record_observation"
            mock_tool_call_1.function.arguments = json.dumps({"id": "obs_1", "payload": {"data": "A"}})
            
            mock_tool_call_2 = MagicMock()
            mock_tool_call_2.function.name = "record_observation"
            mock_tool_call_2.function.arguments = json.dumps({
                "id": "obs_2", 
                "payload": {"data": "B"},
                "justifications": [["obs_1"]]
            })
            
            mock_response.choices = [MagicMock()]
            mock_response.choices[0].message.tool_calls = [mock_tool_call_1, mock_tool_call_2]
            mock_create.return_value = mock_response

            # 2. Execute Intercepted Call
            client = OpenAI(api_key="sk-test", velarix_session_id="test-session")
            client.velarix_client = mock_client
            
            client.chat.completions.create(
                model="gpt-4o",
                messages=[{"role": "user", "content": "Test"}]
            )

            # 3. Verify System Prompt Injection
            args, kwargs = mock_create.call_args
            injected_content = kwargs["messages"][0]["content"]
            assert "VELARIX EPISTEMIC PROTOCOL" in injected_content

            # 4. Verify Tool Assertions via mock_post
            fact_calls = [c for c in mock_post.call_args_list if "/facts" in c.args[0]]
            assert len(fact_calls) == 2
            
            # obs_1 (root)
            assert fact_calls[0].kwargs["json"]["ID"] == "obs_1"
            # obs_2 (derived)
            assert fact_calls[1].kwargs["json"]["ID"] == "obs_2"
            assert fact_calls[1].kwargs["json"]["justification_sets"] == [["obs_1"]]

            print("PASS: test_openai_interceptor_parallel_calls")

def test_openai_overconfidence_downgrade():
    # Mock Velarix Session
    mock_session = MagicMock(spec=VelarixSession)
    mock_session.base_url = "http://localhost:8080/s/test-session"
    mock_session._headers.return_value = {}
    
    # Mock Velarix Client
    mock_client = MagicMock()
    mock_client.session.return_value = mock_session
    mock_client.headers = {}
    
    with patch('openai.resources.chat.completions.Completions.create') as mock_create:
        with patch('requests.post') as mock_post:
            # 1. Setup Mocked OpenAI Response with 0.99 confidence on a root
            mock_response = MagicMock()
            mock_tool_call = MagicMock()
            mock_tool_call.function.name = "record_observation"
            mock_tool_call.function.arguments = json.dumps({
                "id": "too_confident", 
                "payload": {"fact": "I am a god"},
                "confidence": 0.99
            })
            
            mock_response.choices = [MagicMock()]
            mock_response.choices[0].message.tool_calls = [mock_tool_call]
            mock_create.return_value = mock_response

            # 2. Execute
            client = OpenAI(api_key="sk-test", velarix_session_id="test-session")
            client.velarix_client = mock_client
            
            client.chat.completions.create(model="gpt-4o", messages=[{"role": "user", "content": "Test"}])

            # 3. Verify Downgrade
            # Instead of calling observe, the adapter calls requests.post directly for roots with manual status
            # Check the last post call which should be the fact assertion
            fact_assertion_call = [c for c in mock_post.call_args_list if "/facts" in c.args[0]][0]
            assert fact_assertion_call.kwargs["json"]["ManualStatus"] == 0.75
            
            # 4. Verify History Event
            history_call = [c for c in mock_post.call_args_list if "/history" in c.args[0]][0]
            assert history_call.kwargs["json"]["type"] == "confidence_adjusted"
            assert history_call.kwargs["json"]["payload"]["original"] == 0.99

            print("PASS: test_openai_overconfidence_downgrade")

def test_openai_provenance_injection():
    # Mock Session
    mock_session = MagicMock(spec=VelarixSession)
    mock_session.base_url = "http://localhost:8080/s/test-session"
    mock_session._headers.return_value = {}
    mock_session.get_slice.return_value = "## Fact: F1\n```json\n{}\n```"
    
    # Mock Client
    mock_client = MagicMock()
    mock_client.session.return_value = mock_session
    mock_client.headers = {}
    
    with patch('openai.resources.chat.completions.Completions.create') as mock_create:
        with patch('requests.post') as mock_post:
            # 1. Setup Mocked OpenAI Response
            mock_response = MagicMock()
            mock_response.model = "gpt-4o-test-model"
            mock_response.created = 123456789
            
            mock_tool_call = MagicMock()
            mock_tool_call.id = "call_999"
            mock_tool_call.function.name = "record_observation"
            mock_tool_call.function.arguments = json.dumps({
                "id": "prov_test", 
                "payload": {"city": "New York"}
            })
            
            mock_response.choices = [MagicMock()]
            mock_response.choices[0].message.tool_calls = [mock_tool_call]
            mock_create.return_value = mock_response

            # 2. Execute
            client = OpenAI(api_key="sk-test", velarix_session_id="test-session")
            client.velarix_client = mock_client
            client.chat.completions.create(model="gpt-4o", messages=[{"role": "user", "content": "Test"}])

            # 3. Verify Provenance in the POST body
            fact_assertion_call = [c for c in mock_post.call_args_list if "/facts" in c.args[0]][0]
            payload = fact_assertion_call.kwargs["json"]["payload"]
            
            assert "_provenance" in payload
            assert payload["_provenance"]["model"] == "gpt-4o-test-model"
            assert payload["_provenance"]["tool_call_id"] == "call_999"
            assert payload["city"] == "New York"

            print("PASS: test_openai_provenance_injection")

if __name__ == "__main__":
    test_openai_interceptor_parallel_calls()
    test_openai_overconfidence_downgrade()
    test_openai_provenance_injection()

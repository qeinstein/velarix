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
    mock_session.get_slice.return_value = "## Fact: F1\n```json\n{}\n```"
    mock_session.observe.return_value = {}
    mock_session.derive.return_value = {}
    mock_session.append_history.return_value = {}
    mock_session.consistency_check.return_value = {"issue_count": 0, "issues": []}
    
    # Mock Velarix Client
    mock_client = MagicMock()
    mock_client.session.return_value = mock_session
    mock_client.headers = {}
    
    with patch('openai.resources.chat.completions.Completions.create') as mock_create:
        # 1. Setup Mocked OpenAI Response with 2 parallel tool calls
        mock_response = MagicMock()
        mock_tool_call_1 = MagicMock()
        mock_tool_call_1.id = "call_1"
        mock_tool_call_1.function.name = "record_observation"
        mock_tool_call_1.function.arguments = json.dumps({"id": "obs_1", "payload": {"data": "A"}})
        
        mock_tool_call_2 = MagicMock()
        mock_tool_call_2.id = "call_2"
        mock_tool_call_2.function.name = "record_observation"
        mock_tool_call_2.function.arguments = json.dumps({
            "id": "obs_2", 
            "payload": {"data": "B"},
            "justifications": [["obs_1"]]
        })
        
        mock_response.model = "gpt-4o"
        mock_response.created = 123
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

        # 4. Verify persistence through the shared session API
        mock_session.observe.assert_called_once()
        observe_args, observe_kwargs = mock_session.observe.call_args
        assert observe_args[0] == "obs_1"
        assert observe_kwargs.get("confidence", observe_args[2] if len(observe_args) > 2 else None) == 0.75

        history_args, history_kwargs = mock_session.append_history.call_args
        assert history_args[0] == "confidence_adjusted"
        assert history_kwargs["fact_id"] == "obs_1"

        mock_session.derive.assert_called_once()
        derive_args, _ = mock_session.derive.call_args
        assert derive_args[0] == "obs_2"
        assert derive_args[1] == [["obs_1"]]

        print("PASS: test_openai_interceptor_parallel_calls")

def test_openai_overconfidence_downgrade():
    # Mock Velarix Session
    mock_session = MagicMock(spec=VelarixSession)
    mock_session.observe.return_value = {}
    mock_session.append_history.return_value = {}
    mock_session.consistency_check.return_value = {"issue_count": 0, "issues": []}
    
    # Mock Velarix Client
    mock_client = MagicMock()
    mock_client.session.return_value = mock_session
    mock_client.headers = {}
    
    with patch('openai.resources.chat.completions.Completions.create') as mock_create:
        # 1. Setup Mocked OpenAI Response with 0.99 confidence on a root
        mock_response = MagicMock()
        mock_tool_call = MagicMock()
        mock_tool_call.id = "call_confidence"
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
        observe_args, observe_kwargs = mock_session.observe.call_args
        assert observe_args[0] == "too_confident"
        assert observe_kwargs.get("confidence", observe_args[2] if len(observe_args) > 2 else None) == 0.75
        
        # 4. Verify History Event
        history_args, history_kwargs = mock_session.append_history.call_args
        assert history_args[0] == "confidence_adjusted"
        assert history_args[1]["original"] == 0.99
        assert history_kwargs["fact_id"] == "too_confident"

        print("PASS: test_openai_overconfidence_downgrade")

def test_openai_provenance_injection():
    # Mock Session
    mock_session = MagicMock(spec=VelarixSession)
    mock_session.get_slice.return_value = "## Fact: F1\n```json\n{}\n```"
    mock_session.observe.return_value = {}
    mock_session.append_history.return_value = {}
    mock_session.consistency_check.return_value = {"issue_count": 0, "issues": []}
    
    # Mock Client
    mock_client = MagicMock()
    mock_client.session.return_value = mock_session
    mock_client.headers = {}
    
    with patch('openai.resources.chat.completions.Completions.create') as mock_create:
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

        # 3. Verify Provenance in the persisted payload
        observe_args, _ = mock_session.observe.call_args
        payload = observe_args[1]
        
        assert "_provenance" in payload
        assert payload["_provenance"]["model"] == "gpt-4o-test-model"
        assert payload["_provenance"]["tool_call_id"] == "call_999"
        assert payload["city"] == "New York"
        assert mock_session.append_history.call_args.kwargs["fact_id"] == "prov_test"

        print("PASS: test_openai_provenance_injection")

def test_openai_verify_revise_loop():
    mock_session = MagicMock(spec=VelarixSession)
    mock_session.get_slice.return_value = "## Fact: F1\n```json\n{}\n```"
    mock_session.observe.return_value = {}
    mock_session.append_history.return_value = {}
    mock_session.consistency_check.side_effect = [
        {"issue_count": 1, "issues": [{"type": "claim_value_conflict"}]},
        {"issue_count": 0, "issues": []},
    ]

    mock_client = MagicMock()
    mock_client.session.return_value = mock_session
    mock_client.headers = {}

    with patch('openai.resources.chat.completions.Completions.create') as mock_create:
        first_response = MagicMock()
        first_tool_call = MagicMock()
        first_tool_call.id = "call_1"
        first_tool_call.function.name = "record_observation"
        first_tool_call.function.arguments = json.dumps({"id": "obs_conflict", "payload": {"claim_key": "x", "claim_value": "a"}})
        first_response.model = "gpt-4o"
        first_response.created = 123
        first_response.choices = [MagicMock()]
        first_response.choices[0].message.tool_calls = [first_tool_call]

        second_response = MagicMock()
        second_response.model = "gpt-4o"
        second_response.created = 124
        second_response.choices = [MagicMock()]
        second_response.choices[0].message.tool_calls = []

        mock_create.side_effect = [first_response, second_response]

        client = OpenAI(api_key="sk-test", velarix_session_id="test-session", velarix_verify_rounds=1)
        client.velarix_client = mock_client
        result = client.chat.completions.create(model="gpt-4o", messages=[{"role": "user", "content": "Test"}])

        assert result is second_response
        assert mock_create.call_count == 2
        _, second_kwargs = mock_create.call_args_list[1]
        assert any("Velarix verification failed" in msg["content"] for msg in second_kwargs["messages"] if msg["role"] == "user")

        print("PASS: test_openai_verify_revise_loop")

if __name__ == "__main__":
    test_openai_interceptor_parallel_calls()
    test_openai_overconfidence_downgrade()
    test_openai_provenance_injection()
    test_openai_verify_revise_loop()

import os
import sys
from unittest.mock import MagicMock

import pytest

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

try:
    from langchain_core.messages import AIMessage, HumanMessage
except ImportError:
    AIMessage = None
    HumanMessage = None

from velarix.client import VelarixSession
from velarix.integrations.langchain import VelarixLangChainChatModel


class FakeToolCapableModel:
    def __init__(self):
        self.bound_tools = []
        self.bound_kwargs = {}
        self.invocations = []
        self.model_name = "fake-langchain-model"

    def bind_tools(self, tools, **kwargs):
        self.bound_tools = list(tools)
        self.bound_kwargs = dict(kwargs)
        return self

    def invoke(self, messages, **kwargs):
        self.invocations.append((messages, kwargs))
        return AIMessage(
            content="stored",
            tool_calls=[
                {
                    "name": "record_observation",
                    "args": {"id": "lc_obs", "payload": {"topic": "LangChain"}},
                    "id": "tool_call_1",
                    "type": "tool_call",
                }
            ],
        )


def test_langchain_wrapper_injects_runtime_and_persists():
    if AIMessage is None or HumanMessage is None:
        pytest.skip("LangChain dependencies not installed")

    mock_session = MagicMock(spec=VelarixSession)
    mock_session.get_slice.return_value = "## Fact: LC1\n```json\n{}\n```"
    mock_session.observe.return_value = {}
    mock_session.append_history.return_value = {}

    wrapped_model = FakeToolCapableModel()
    model = VelarixLangChainChatModel(model=wrapped_model, session=mock_session)

    result = model.invoke([HumanMessage(content="Test LangChain")])

    assert isinstance(result, AIMessage)
    assert any(tool["function"]["name"] == "record_observation" for tool in wrapped_model.bound_tools)
    assert any(tool["function"]["name"] == "explain_reasoning" for tool in wrapped_model.bound_tools)

    invoke_messages, invoke_kwargs = wrapped_model.invocations[0]
    assert "VELARIX EPISTEMIC PROTOCOL" in invoke_messages[0].content
    assert "tools" not in invoke_kwargs
    assert "tool_choice" not in invoke_kwargs

    observe_args, observe_kwargs = mock_session.observe.call_args
    assert observe_args[0] == "lc_obs"
    assert observe_args[1]["topic"] == "LangChain"
    assert observe_args[1]["_provenance"]["source"] == "langchain_adapter"
    assert observe_kwargs.get("confidence", observe_args[2] if len(observe_args) > 2 else None) == 0.75
    assert mock_session.append_history.call_args.kwargs["fact_id"] == "lc_obs"

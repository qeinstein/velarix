import json
import time
from types import SimpleNamespace
from typing import Any, Dict, List, Optional

from velarix.runtime import VelarixChatRuntime

try:
    from pydantic import ConfigDict
    from langchain_core.language_models.chat_models import BaseChatModel
    from langchain_core.messages import AIMessage, BaseMessage, ChatMessage, HumanMessage, SystemMessage, ToolMessage
    from langchain_core.outputs import ChatGeneration, ChatResult
except ImportError as exc:  # pragma: no cover - optional dependency
    _LANGCHAIN_IMPORT_ERROR = exc

    class VelarixLangChainChatModel:  # type: ignore[no-redef]
        def __init__(self, *_args, **_kwargs):
            raise ImportError(
                "LangChain support requires optional dependencies. Install with `pip install velarix[langchain]`."
            ) from _LANGCHAIN_IMPORT_ERROR

else:
    class VelarixLangChainChatModel(BaseChatModel):
        model: Any
        session: Any
        source: str = "langchain_adapter"
        strict: bool = True
        parallel_tool_calls: Optional[bool] = None

        model_config = ConfigDict(arbitrary_types_allowed=True)

        @property
        def _llm_type(self) -> str:
            return "velarix_langchain"

        @property
        def _identifying_params(self) -> Dict[str, Any]:
            wrapped_type = getattr(self.model, "_llm_type", None)
            if callable(wrapped_type):
                try:
                    wrapped_type = wrapped_type()
                except TypeError:
                    wrapped_type = None
            return {
                "source": self.source,
                "wrapped_model_type": wrapped_type or self.model.__class__.__name__,
            }

        def _generate(
            self,
            messages: List[BaseMessage],
            stop: Optional[List[str]] = None,
            run_manager: Optional[Any] = None,
            **kwargs: Any,
        ) -> ChatResult:
            runtime = VelarixChatRuntime(self.session, source=self.source, strict=self.strict)
            prepared = runtime.prepare_params(self._prepare_runtime_input(messages, kwargs))
            bound_model = self._bind_wrapped_model(prepared.get("tools"), prepared.get("tool_choice"))
            invoke_kwargs = self._prepare_invoke_kwargs(kwargs, stop, run_manager)
            response_message = bound_model.invoke(self._to_langchain_messages(prepared["messages"]), **invoke_kwargs)
            runtime.process_response(self._to_openai_like_response(response_message))
            return ChatResult(generations=[ChatGeneration(message=response_message)])

        async def _agenerate(
            self,
            messages: List[BaseMessage],
            stop: Optional[List[str]] = None,
            run_manager: Optional[Any] = None,
            **kwargs: Any,
        ) -> ChatResult:
            runtime = VelarixChatRuntime(self.session, source=self.source, strict=self.strict)
            prepared = runtime.prepare_params(self._prepare_runtime_input(messages, kwargs))
            bound_model = self._bind_wrapped_model(prepared.get("tools"), prepared.get("tool_choice"))
            invoke_kwargs = self._prepare_invoke_kwargs(kwargs, stop, run_manager)
            if not hasattr(bound_model, "ainvoke"):
                raise TypeError("Wrapped LangChain model does not support ainvoke()")
            response_message = await bound_model.ainvoke(self._to_langchain_messages(prepared["messages"]), **invoke_kwargs)
            runtime.process_response(self._to_openai_like_response(response_message))
            return ChatResult(generations=[ChatGeneration(message=response_message)])

        def _prepare_runtime_input(self, messages: List[BaseMessage], kwargs: Dict[str, Any]) -> Dict[str, Any]:
            return {
                "messages": [self._to_openai_message(message) for message in messages],
                "tools": kwargs.get("tools"),
                "tool_choice": kwargs.get("tool_choice"),
            }

        def _prepare_invoke_kwargs(
            self,
            kwargs: Dict[str, Any],
            stop: Optional[List[str]],
            run_manager: Optional[Any],
        ) -> Dict[str, Any]:
            invoke_kwargs = {k: v for k, v in kwargs.items() if k not in {"tools", "tool_choice"}}
            if stop is not None:
                invoke_kwargs["stop"] = stop
            if run_manager is not None:
                config = dict(invoke_kwargs.get("config") or {})
                config.setdefault("callbacks", run_manager)
                invoke_kwargs["config"] = config
            return invoke_kwargs

        def _bind_wrapped_model(self, tools: Optional[List[Dict[str, Any]]], tool_choice: Optional[Any]) -> Any:
            if not hasattr(self.model, "bind_tools"):
                raise TypeError("Wrapped LangChain model must support bind_tools()")
            bind_kwargs: Dict[str, Any] = {}
            if tool_choice is not None:
                bind_kwargs["tool_choice"] = tool_choice
            if self.parallel_tool_calls is not None:
                bind_kwargs["parallel_tool_calls"] = self.parallel_tool_calls
            return self.model.bind_tools(tools or [], **bind_kwargs)

        def _to_openai_message(self, message: BaseMessage) -> Dict[str, Any]:
            payload: Dict[str, Any] = {"content": getattr(message, "content", "")}
            if isinstance(message, HumanMessage):
                payload["role"] = "user"
            elif isinstance(message, SystemMessage):
                payload["role"] = "system"
            elif isinstance(message, ToolMessage):
                payload["role"] = "tool"
                payload["tool_call_id"] = getattr(message, "tool_call_id", None)
            elif isinstance(message, AIMessage):
                payload["role"] = "assistant"
                tool_calls = []
                for tool_call in getattr(message, "tool_calls", []) or []:
                    tool_calls.append(
                        {
                            "id": tool_call.get("id"),
                            "type": "function",
                            "function": {
                                "name": tool_call.get("name"),
                                "arguments": json.dumps(tool_call.get("args") or {}),
                            },
                        }
                    )
                if tool_calls:
                    payload["tool_calls"] = tool_calls
            elif isinstance(message, ChatMessage):
                payload["role"] = message.role
            else:
                payload["role"] = "user"
            return payload

        def _to_langchain_messages(self, messages: List[Dict[str, Any]]) -> List[BaseMessage]:
            converted: List[BaseMessage] = []
            for message in messages:
                role = message.get("role")
                content = message.get("content", "")
                if role == "system":
                    converted.append(SystemMessage(content=content))
                elif role == "assistant":
                    converted.append(
                        AIMessage(
                            content=content,
                            tool_calls=[self._to_langchain_tool_call(tc) for tc in message.get("tool_calls", [])],
                        )
                    )
                elif role == "tool":
                    converted.append(ToolMessage(content=content, tool_call_id=message.get("tool_call_id") or ""))
                elif role == "user":
                    converted.append(HumanMessage(content=content))
                else:
                    converted.append(ChatMessage(role=role or "user", content=content))
            return converted

        def _to_langchain_tool_call(self, tool_call: Dict[str, Any]) -> Dict[str, Any]:
            function = tool_call.get("function", {})
            args = function.get("arguments", {})
            if isinstance(args, str):
                try:
                    args = json.loads(args)
                except json.JSONDecodeError:
                    args = {"raw_arguments": args}
            return {
                "name": function.get("name"),
                "args": args,
                "id": tool_call.get("id"),
                "type": "tool_call",
            }

        def _to_openai_like_response(self, message: BaseMessage) -> Any:
            tool_calls = []
            for tool_call in getattr(message, "tool_calls", []) or []:
                tool_calls.append(
                    SimpleNamespace(
                        id=tool_call.get("id"),
                        function=SimpleNamespace(
                            name=tool_call.get("name"),
                            arguments=json.dumps(tool_call.get("args") or {}),
                        ),
                    )
                )
            return SimpleNamespace(
                model=getattr(self.model, "model_name", self.model.__class__.__name__),
                created=int(time.time()),
                choices=[SimpleNamespace(message=SimpleNamespace(tool_calls=tool_calls))],
            )


    def wrap_langchain_model(
        model: Any,
        session: Any,
        source: str = "langchain_adapter",
        strict: bool = True,
        parallel_tool_calls: Optional[bool] = None,
    ) -> VelarixLangChainChatModel:
        return VelarixLangChainChatModel(
            model=model,
            session=session,
            source=source,
            strict=strict,
            parallel_tool_calls=parallel_tool_calls,
        )

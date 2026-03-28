import json
from copy import deepcopy
from typing import Any, Dict, List, Optional


DEFAULT_SYSTEM_PROMPT = "You are a helpful assistant."
RECORD_OBSERVATION_TOOL = "record_observation"
EXPLAIN_REASONING_TOOL = "explain_reasoning"


def build_system_instruction(context_markdown: str) -> str:
    return (
        "\n\n## VELARIX EPISTEMIC PROTOCOL\n"
        "You are equipped with a memory layer (Velarix). Below are the current justified beliefs in this session. "
        f"Use the '{RECORD_OBSERVATION_TOOL}' tool whenever you derive, infer, or assert a new fact that should be "
        "remembered. If your observation depends on any current beliefs, use their exact IDs (e.g., 'fact_123') "
        "in the 'justifications' field. Use an OR-of-ANDs format: [[id1, id2], [id3]].\n\n"
        "When the user asks you to explain a decision, justify a recommendation, or describe your reasoning, "
        f"ALWAYS use the '{EXPLAIN_REASONING_TOOL}' tool. Never invent an explanation; narrate exactly what the tool "
        "returns, respecting confidence tiers and provenance.\n\n"
        "## CURRENT BELIEFS (Velarix)\n"
        f"{context_markdown}\n"
        "---\n"
    )


def velarix_tools() -> List[Dict[str, Any]]:
    return [
        {
            "type": "function",
            "function": {
                "name": RECORD_OBSERVATION_TOOL,
                "description": "Persist a new justified belief, observation, or derived plan into long-term memory.",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "id": {"type": "string", "description": "A unique, slugified identifier."},
                        "payload": {"type": "object", "description": "The JSON data associated with this fact."},
                        "justifications": {
                            "type": "array",
                            "items": {"type": "array", "items": {"type": "string"}},
                            "description": "List of lists of Fact IDs that justify this observation.",
                        },
                        "confidence": {"type": "number", "description": "Your confidence (0.0 to 1.0)."},
                    },
                    "required": ["id", "payload"],
                },
            },
        },
        {
            "type": "function",
            "function": {
                "name": EXPLAIN_REASONING_TOOL,
                "description": (
                    "Call this tool when the user asks you to explain a decision, justify a recommendation, "
                    "or describe your reasoning at any point in the conversation."
                ),
                "parameters": {
                    "type": "object",
                    "properties": {
                        "fact_id": {
                            "type": "string",
                            "description": "The specific belief/fact ID to explain. If omitted, explains the most recent decision.",
                        },
                        "timestamp": {
                            "type": "string",
                            "description": "ISO8601 timestamp; explain the reasoning state at that exact point in time, not the current state.",
                        },
                        "counterfactual_fact_id": {
                            "type": "string",
                            "description": "A fact ID to hypothetically remove. The explanation will describe what would have been different.",
                        },
                    },
                },
            },
        },
    ]


def merge_tools(existing_tools: Optional[List[Dict[str, Any]]]) -> List[Dict[str, Any]]:
    merged = list(existing_tools or [])
    existing_names = {
        tool.get("function", {}).get("name")
        for tool in merged
        if isinstance(tool, dict)
    }
    for tool in velarix_tools():
        if tool["function"]["name"] not in existing_names:
            merged.append(tool)
    return merged


def inject_system_instruction(messages: Optional[List[Dict[str, Any]]], context_markdown: str) -> List[Dict[str, Any]]:
    prepared = [deepcopy(message) for message in (messages or [])]
    instruction = build_system_instruction(context_markdown)
    for message in prepared:
        if message.get("role") == "system":
            message["content"] = f"{message.get('content', '')}{instruction}"
            return prepared
    prepared.insert(0, {"role": "system", "content": DEFAULT_SYSTEM_PROMPT + instruction})
    return prepared


class VelarixChatRuntime:
    def __init__(self, session: Any, source: str, strict: bool = True):
        self.session = session
        self.source = source
        self.strict = strict

    def prepare_params(self, params: Dict[str, Any]) -> Dict[str, Any]:
        prepared = dict(params)
        context_markdown = self.session.get_slice(format="markdown")
        prepared["messages"] = inject_system_instruction(prepared.get("messages"), context_markdown)
        prepared["tools"] = merge_tools(prepared.get("tools"))
        if "tool_choice" not in prepared:
            prepared["tool_choice"] = "auto"
        return prepared

    def process_response(self, response: Any) -> Any:
        errors: List[str] = []
        for tool_call in _iter_tool_calls(response):
            name = getattr(getattr(tool_call, "function", None), "name", "tool")
            try:
                if name == RECORD_OBSERVATION_TOOL:
                    self._persist_observation(tool_call, response)
                elif name == EXPLAIN_REASONING_TOOL:
                    self._resolve_explanation(tool_call)
            except Exception as exc:
                errors.append(f"{name}: {exc}")
        if errors and self.strict:
            raise RuntimeError("; ".join(errors))
        return response

    def _persist_observation(self, tool_call: Any, response: Any) -> None:
        args = json.loads(tool_call.function.arguments)
        fact_id = args.get("id")
        if not fact_id:
            raise ValueError("record_observation requires an id")

        payload = args.get("payload") or {}
        justifications = args.get("justifications")
        confidence = float(args.get("confidence", 1.0))
        payload["_provenance"] = {
            "source": self.source,
            "model": getattr(response, "model", None),
            "timestamp": getattr(response, "created", None),
            "tool_call_id": getattr(tool_call, "id", None),
        }

        is_root = not justifications
        if is_root and confidence > 0.9:
            self.session.append_history(
                "confidence_adjusted",
                {"original": confidence, "adjusted": 0.75},
                fact_id=fact_id,
            )
            confidence = 0.75

        if justifications:
            self.session.derive(fact_id, justifications, payload)
        else:
            self.session.observe(fact_id, payload, confidence=confidence)

    def _resolve_explanation(self, tool_call: Any) -> None:
        args = json.loads(tool_call.function.arguments)
        explanation = self.session.explain(
            fact_id=args.get("fact_id"),
            timestamp=args.get("timestamp"),
            counterfactual_fact_id=args.get("counterfactual_fact_id"),
        )
        tool_call.function.arguments = json.dumps(explanation)


class AsyncVelarixChatRuntime:
    def __init__(self, session: Any, source: str, strict: bool = True):
        self.session = session
        self.source = source
        self.strict = strict

    async def prepare_params(self, params: Dict[str, Any]) -> Dict[str, Any]:
        prepared = dict(params)
        context_markdown = await self.session.get_slice(format="markdown")
        prepared["messages"] = inject_system_instruction(prepared.get("messages"), context_markdown)
        prepared["tools"] = merge_tools(prepared.get("tools"))
        if "tool_choice" not in prepared:
            prepared["tool_choice"] = "auto"
        return prepared

    async def process_response(self, response: Any) -> Any:
        errors: List[str] = []
        for tool_call in _iter_tool_calls(response):
            name = getattr(getattr(tool_call, "function", None), "name", "tool")
            try:
                if name == RECORD_OBSERVATION_TOOL:
                    await self._persist_observation(tool_call, response)
                elif name == EXPLAIN_REASONING_TOOL:
                    await self._resolve_explanation(tool_call)
            except Exception as exc:
                errors.append(f"{name}: {exc}")
        if errors and self.strict:
            raise RuntimeError("; ".join(errors))
        return response

    async def _persist_observation(self, tool_call: Any, response: Any) -> None:
        args = json.loads(tool_call.function.arguments)
        fact_id = args.get("id")
        if not fact_id:
            raise ValueError("record_observation requires an id")

        payload = args.get("payload") or {}
        justifications = args.get("justifications")
        confidence = float(args.get("confidence", 1.0))
        payload["_provenance"] = {
            "source": self.source,
            "model": getattr(response, "model", None),
            "timestamp": getattr(response, "created", None),
            "tool_call_id": getattr(tool_call, "id", None),
        }

        is_root = not justifications
        if is_root and confidence > 0.9:
            await self.session.append_history(
                "confidence_adjusted",
                {"original": confidence, "adjusted": 0.75},
                fact_id=fact_id,
            )
            confidence = 0.75

        if justifications:
            await self.session.derive(fact_id, justifications, payload)
        else:
            await self.session.observe(fact_id, payload, confidence=confidence)

    async def _resolve_explanation(self, tool_call: Any) -> None:
        args = json.loads(tool_call.function.arguments)
        explanation = await self.session.explain(
            fact_id=args.get("fact_id"),
            timestamp=args.get("timestamp"),
            counterfactual_fact_id=args.get("counterfactual_fact_id"),
        )
        tool_call.function.arguments = json.dumps(explanation)


def _iter_tool_calls(response: Any) -> List[Any]:
    choices = getattr(response, "choices", None) or []
    if not choices:
        return []
    message = getattr(choices[0], "message", None)
    return getattr(message, "tool_calls", None) or []

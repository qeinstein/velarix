import json
from copy import deepcopy
from typing import Any, Dict, List, Optional


DEFAULT_SYSTEM_PROMPT = "You are a helpful assistant."
RECORD_OBSERVATION_TOOL = "record_observation"
EXPLAIN_REASONING_TOOL = "explain_reasoning"
RECORD_PERCEPTION_TOOL = "record_perception"
SEMANTIC_MEMORY_SEARCH_TOOL = "semantic_memory_search"
RECORD_REASONING_CHAIN_TOOL = "record_reasoning_chain"
VERIFY_REASONING_CHAIN_TOOL = "verify_reasoning_chain"
CONSISTENCY_CHECK_TOOL = "consistency_check"


def _report_issue_count(report: Any) -> int:
    if isinstance(report, dict):
        value = report.get("issue_count", 0)
        if isinstance(value, (int, float)):
            return int(value)
    return 0


def _audit_is_valid(report: Any) -> bool:
    if isinstance(report, dict):
        return bool(report.get("valid", True))
    return True


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
        f"For perceptual or model-derived observations, use '{RECORD_PERCEPTION_TOOL}' so the observation is stored as "
        "a first-class belief with optional modality, provider, and embedding metadata.\n\n"
        f"For multi-step deliberation, use '{RECORD_REASONING_CHAIN_TOOL}' to persist the chain, and use "
        f"'{VERIFY_REASONING_CHAIN_TOOL}' or '{CONSISTENCY_CHECK_TOOL}' before relying on a chain for a high-impact action. "
        f"Use '{SEMANTIC_MEMORY_SEARCH_TOOL}' when you need semantically related beliefs instead of only direct justifications.\n\n"
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
                "name": RECORD_PERCEPTION_TOOL,
                "description": "Persist a neural or perceptual observation as a root belief with optional embedding metadata.",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "id": {"type": "string"},
                        "payload": {"type": "object"},
                        "confidence": {"type": "number"},
                        "modality": {"type": "string"},
                        "provider": {"type": "string"},
                        "model": {"type": "string"},
                        "embedding": {"type": "array", "items": {"type": "number"}},
                        "metadata": {"type": "object"},
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
        {
            "type": "function",
            "function": {
                "name": SEMANTIC_MEMORY_SEARCH_TOOL,
                "description": "Retrieve semantically related facts from the belief store.",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "query": {"type": "string"},
                        "limit": {"type": "integer"},
                        "valid_only": {"type": "boolean"},
                    },
                    "required": ["query"],
                },
            },
        },
        {
            "type": "function",
            "function": {
                "name": RECORD_REASONING_CHAIN_TOOL,
                "description": "Persist a multi-step reasoning chain with explicit evidence and output fact links.",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "chain_id": {"type": "string"},
                        "model": {"type": "string"},
                        "mode": {"type": "string"},
                        "summary": {"type": "string"},
                        "steps": {
                            "type": "array",
                            "items": {
                                "type": "object",
                                "properties": {
                                    "id": {"type": "string"},
                                    "kind": {"type": "string"},
                                    "content": {"type": "string"},
                                    "evidence_fact_ids": {"type": "array", "items": {"type": "string"}},
                                    "justification_fact_ids": {"type": "array", "items": {"type": "string"}},
                                    "output_fact_id": {"type": "string"},
                                    "contradicts_fact_ids": {"type": "array", "items": {"type": "string"}},
                                    "confidence": {"type": "number"},
                                },
                                "required": ["content"],
                            },
                        },
                    },
                    "required": ["steps"],
                },
            },
        },
        {
            "type": "function",
            "function": {
                "name": VERIFY_REASONING_CHAIN_TOOL,
                "description": "Audit a stored reasoning chain and optionally retract contradicted earlier facts.",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "chain_id": {"type": "string"},
                        "auto_retract": {"type": "boolean"},
                    },
                    "required": ["chain_id"],
                },
            },
        },
        {
            "type": "function",
            "function": {
                "name": CONSISTENCY_CHECK_TOOL,
                "description": "Run a contradiction and belief-consistency scan over a set of facts.",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "fact_ids": {"type": "array", "items": {"type": "string"}},
                        "max_facts": {"type": "integer"},
                        "include_invalid": {"type": "boolean"},
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
        self.last_processed: Dict[str, Any] = {}

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
        self.last_processed = {
            "fact_ids": [],
            "reasoning_chain_ids": [],
            "verification_reports": [],
            "consistency_reports": [],
        }
        for tool_call in _iter_tool_calls(response):
            name = getattr(getattr(tool_call, "function", None), "name", "tool")
            try:
                if name == RECORD_OBSERVATION_TOOL:
                    self._persist_observation(tool_call, response)
                elif name == RECORD_PERCEPTION_TOOL:
                    self._persist_perception(tool_call)
                elif name == EXPLAIN_REASONING_TOOL:
                    self._resolve_explanation(tool_call)
                elif name == SEMANTIC_MEMORY_SEARCH_TOOL:
                    self._resolve_semantic_search(tool_call)
                elif name == RECORD_REASONING_CHAIN_TOOL:
                    self._persist_reasoning_chain(tool_call)
                elif name == VERIFY_REASONING_CHAIN_TOOL:
                    self._resolve_reasoning_verification(tool_call)
                elif name == CONSISTENCY_CHECK_TOOL:
                    self._resolve_consistency_check(tool_call)
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
        self.last_processed["fact_ids"].append(fact_id)

    def _resolve_explanation(self, tool_call: Any) -> None:
        args = json.loads(tool_call.function.arguments)
        explanation = self.session.explain(
            fact_id=args.get("fact_id"),
            timestamp=args.get("timestamp"),
            counterfactual_fact_id=args.get("counterfactual_fact_id"),
        )
        tool_call.function.arguments = json.dumps(explanation)

    def _persist_perception(self, tool_call: Any) -> None:
        args = json.loads(tool_call.function.arguments)
        fact_id = args.get("id")
        if not fact_id:
            raise ValueError("record_perception requires an id")
        self.session.record_perception(
            fact_id,
            args.get("payload") or {},
            confidence=float(args.get("confidence", 0.75)),
            modality=args.get("modality"),
            provider=args.get("provider"),
            model=args.get("model"),
            embedding=args.get("embedding"),
            metadata=args.get("metadata"),
        )
        self.last_processed["fact_ids"].append(fact_id)

    def _resolve_semantic_search(self, tool_call: Any) -> None:
        args = json.loads(tool_call.function.arguments)
        results = self.session.semantic_search(
            args.get("query", ""),
            limit=int(args.get("limit", 10)),
            valid_only=bool(args.get("valid_only", True)),
        )
        tool_call.function.arguments = json.dumps(results)

    def _persist_reasoning_chain(self, tool_call: Any) -> None:
        args = json.loads(tool_call.function.arguments)
        chain = {
            "chain_id": args.get("chain_id"),
            "model": args.get("model"),
            "mode": args.get("mode"),
            "summary": args.get("summary"),
            "steps": args.get("steps") or [],
        }
        stored = self.session.record_reasoning_chain(chain)
        tool_call.function.arguments = json.dumps(stored)
        if stored.get("chain_id"):
            self.last_processed["reasoning_chain_ids"].append(stored["chain_id"])

    def _resolve_reasoning_verification(self, tool_call: Any) -> None:
        args = json.loads(tool_call.function.arguments)
        report = self.session.verify_reasoning_chain(
            args.get("chain_id"),
            auto_retract=bool(args.get("auto_retract", False)),
        )
        tool_call.function.arguments = json.dumps(report)
        self.last_processed["verification_reports"].append(report)

    def _resolve_consistency_check(self, tool_call: Any) -> None:
        args = json.loads(tool_call.function.arguments)
        report = self.session.consistency_check(
            fact_ids=args.get("fact_ids"),
            max_facts=args.get("max_facts"),
            include_invalid=bool(args.get("include_invalid", False)),
        )
        tool_call.function.arguments = json.dumps(report)
        self.last_processed["consistency_reports"].append(report)

    def verify_recent_reasoning(self, auto_retract: bool = True) -> Dict[str, Any]:
        fact_ids = list(dict.fromkeys(self.last_processed.get("fact_ids", [])))
        chain_ids = list(dict.fromkeys(self.last_processed.get("reasoning_chain_ids", [])))
        consistency = None
        if fact_ids:
            consistency = self.session.consistency_check(fact_ids=fact_ids, include_invalid=False)
        audits = []
        for chain_id in chain_ids:
            audits.append(self.session.verify_reasoning_chain(chain_id, auto_retract=auto_retract))
        has_issues = _report_issue_count(consistency) > 0 or any(not _audit_is_valid(audit) for audit in audits)
        summary = {"fact_ids": fact_ids, "reasoning_chain_ids": chain_ids, "consistency": consistency, "audits": audits, "has_issues": has_issues}
        self.last_processed["verification"] = summary
        return summary


class AsyncVelarixChatRuntime:
    def __init__(self, session: Any, source: str, strict: bool = True):
        self.session = session
        self.source = source
        self.strict = strict
        self.last_processed: Dict[str, Any] = {}

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
        self.last_processed = {
            "fact_ids": [],
            "reasoning_chain_ids": [],
            "verification_reports": [],
            "consistency_reports": [],
        }
        for tool_call in _iter_tool_calls(response):
            name = getattr(getattr(tool_call, "function", None), "name", "tool")
            try:
                if name == RECORD_OBSERVATION_TOOL:
                    await self._persist_observation(tool_call, response)
                elif name == RECORD_PERCEPTION_TOOL:
                    await self._persist_perception(tool_call)
                elif name == EXPLAIN_REASONING_TOOL:
                    await self._resolve_explanation(tool_call)
                elif name == SEMANTIC_MEMORY_SEARCH_TOOL:
                    await self._resolve_semantic_search(tool_call)
                elif name == RECORD_REASONING_CHAIN_TOOL:
                    await self._persist_reasoning_chain(tool_call)
                elif name == VERIFY_REASONING_CHAIN_TOOL:
                    await self._resolve_reasoning_verification(tool_call)
                elif name == CONSISTENCY_CHECK_TOOL:
                    await self._resolve_consistency_check(tool_call)
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
        self.last_processed["fact_ids"].append(fact_id)

    async def _resolve_explanation(self, tool_call: Any) -> None:
        args = json.loads(tool_call.function.arguments)
        explanation = await self.session.explain(
            fact_id=args.get("fact_id"),
            timestamp=args.get("timestamp"),
            counterfactual_fact_id=args.get("counterfactual_fact_id"),
        )
        tool_call.function.arguments = json.dumps(explanation)

    async def _persist_perception(self, tool_call: Any) -> None:
        args = json.loads(tool_call.function.arguments)
        fact_id = args.get("id")
        if not fact_id:
            raise ValueError("record_perception requires an id")
        await self.session.record_perception(
            fact_id,
            args.get("payload") or {},
            confidence=float(args.get("confidence", 0.75)),
            modality=args.get("modality"),
            provider=args.get("provider"),
            model=args.get("model"),
            embedding=args.get("embedding"),
            metadata=args.get("metadata"),
        )
        self.last_processed["fact_ids"].append(fact_id)

    async def _resolve_semantic_search(self, tool_call: Any) -> None:
        args = json.loads(tool_call.function.arguments)
        results = await self.session.semantic_search(
            args.get("query", ""),
            limit=int(args.get("limit", 10)),
            valid_only=bool(args.get("valid_only", True)),
        )
        tool_call.function.arguments = json.dumps(results)

    async def _persist_reasoning_chain(self, tool_call: Any) -> None:
        args = json.loads(tool_call.function.arguments)
        chain = {
            "chain_id": args.get("chain_id"),
            "model": args.get("model"),
            "mode": args.get("mode"),
            "summary": args.get("summary"),
            "steps": args.get("steps") or [],
        }
        stored = await self.session.record_reasoning_chain(chain)
        tool_call.function.arguments = json.dumps(stored)
        if stored.get("chain_id"):
            self.last_processed["reasoning_chain_ids"].append(stored["chain_id"])

    async def _resolve_reasoning_verification(self, tool_call: Any) -> None:
        args = json.loads(tool_call.function.arguments)
        report = await self.session.verify_reasoning_chain(
            args.get("chain_id"),
            auto_retract=bool(args.get("auto_retract", False)),
        )
        tool_call.function.arguments = json.dumps(report)
        self.last_processed["verification_reports"].append(report)

    async def _resolve_consistency_check(self, tool_call: Any) -> None:
        args = json.loads(tool_call.function.arguments)
        report = await self.session.consistency_check(
            fact_ids=args.get("fact_ids"),
            max_facts=args.get("max_facts"),
            include_invalid=bool(args.get("include_invalid", False)),
        )
        tool_call.function.arguments = json.dumps(report)
        self.last_processed["consistency_reports"].append(report)

    async def verify_recent_reasoning(self, auto_retract: bool = True) -> Dict[str, Any]:
        fact_ids = list(dict.fromkeys(self.last_processed.get("fact_ids", [])))
        chain_ids = list(dict.fromkeys(self.last_processed.get("reasoning_chain_ids", [])))
        consistency = None
        if fact_ids:
            consistency = await self.session.consistency_check(fact_ids=fact_ids, include_invalid=False)
        audits = []
        for chain_id in chain_ids:
            audits.append(await self.session.verify_reasoning_chain(chain_id, auto_retract=auto_retract))
        has_issues = _report_issue_count(consistency) > 0 or any(not _audit_is_valid(audit) for audit in audits)
        summary = {"fact_ids": fact_ids, "reasoning_chain_ids": chain_ids, "consistency": consistency, "audits": audits, "has_issues": has_issues}
        self.last_processed["verification"] = summary
        return summary


def _iter_tool_calls(response: Any) -> List[Any]:
    choices = getattr(response, "choices", None) or []
    if not choices:
        return []
    message = getattr(choices[0], "message", None)
    return getattr(message, "tool_calls", None) or []

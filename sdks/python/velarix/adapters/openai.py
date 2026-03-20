import json
import os
import requests
import httpx
from typing import Optional, List, Dict, Any, Union
from openai import OpenAI as BaseOpenAI, AsyncOpenAI as BaseAsyncOpenAI
from velarix.client import VelarixClient, AsyncVelarixClient

class OpenAI(BaseOpenAI):
    """
    A drop-in replacement for the OpenAI client that automatically 
    injects Velarix context and extracts facts.
    """
    def __init__(self, *args, velarix_base_url: Optional[str] = None, velarix_session_id: Optional[str] = None, **kwargs):
        super().__init__(*args, **kwargs)
        base_url = velarix_base_url or os.getenv("VELARIX_BASE_URL", "http://localhost:8080")
        self.velarix_client = VelarixClient(base_url=base_url)
        self.velarix_session_id = velarix_session_id

    @property
    def chat(self):
        return VelarixChat(self)

class VelarixChat:
    def __init__(self, client: OpenAI):
        self.client = client

    @property
    def completions(self):
        return VelarixCompletions(self.client)

class VelarixCompletions:
    def __init__(self, client: OpenAI):
        self.client = client
        self._base_completions = super(OpenAI, client).chat.completions

    def create(self, *args, **kwargs):
        session_id = kwargs.pop("velarix_session_id", self.client.velarix_session_id)
        if not session_id:
            return self._base_completions.create(*args, **kwargs)

        session = self.client.velarix_client.session(session_id)

        messages = kwargs.get("messages", [])
        context_markdown = session.get_slice(format="markdown")
        
        system_instruction = (
            "\n\n## VELARIX EPISTEMIC PROTOCOL\n"
            "You are equipped with a memory layer (Velarix). Below are the current justified beliefs in this session. "
            "Use the 'record_observation' tool whenever you derive, infer, or assert a new fact that should be "
            "remembered. If your observation depends on any current beliefs, use their exact IDs (e.g., 'fact_123') "
            "in the 'justifications' field. Use an OR-of-ANDs format: [[id1, id2], [id3]].\n\n"
            "## CURRENT BELIEFS (Velarix)\n"
            f"{context_markdown}\n"
            "---\n"
        )

        system_msg = next((m for m in messages if m["role"] == "system"), None)
        if system_msg:
            system_msg["content"] = str(system_msg["content"]) + system_instruction
        else:
            messages.insert(0, {"role": "system", "content": "You are a helpful assistant." + system_instruction})

        tools = kwargs.get("tools", [])
        observation_tool = {
            "type": "function",
            "function": {
                "name": "record_observation",
                "description": "Persist a new justified belief, observation, or derived plan into long-term memory.",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "id": {"type": "string", "description": "A unique, slugified identifier."},
                        "payload": {"type": "object", "description": "The JSON data associated with this fact."},
                        "justifications": {
                            "type": "array",
                            "items": {"type": "array", "items": {"type": "string"}},
                            "description": "List of lists of Fact IDs that justify this observation."
                        },
                        "confidence": {"type": "number", "description": "Your confidence (0.0 to 1.0)."}
                    },
                    "required": ["id", "payload"]
                }
            }
        }
        tools.append(observation_tool)
        kwargs["tools"] = tools
        if "tool_choice" not in kwargs:
            kwargs["tool_choice"] = "auto"

        response = self._base_completions.create(*args, **kwargs)

        choice = response.choices[0]
        if choice.message.tool_calls:
            for tool_call in choice.message.tool_calls:
                if tool_call.function.name == "record_observation":
                    try:
                        args = json.loads(tool_call.function.arguments)
                        fact_id = args.get("id")
                        payload = args.get("payload") or {}
                        justifications = args.get("justifications")
                        confidence = args.get("confidence", 1.0)

                        payload["_provenance"] = {
                            "source": "openai_interceptor",
                            "model": response.model,
                            "timestamp": response.created,
                            "tool_call_id": tool_call.id
                        }

                        is_root = not justifications or len(justifications) == 0
                        if is_root and confidence > 0.9:
                            try:
                                session.client.headers.update({"X-Velarix-Event": "confidence_adjusted"})
                            except AttributeError:
                                pass

                            requests.post(
                                f"{session.base_url}/history", 
                                json={
                                    "type": "confidence_adjusted",
                                    "session_id": session_id,
                                    "fact_id": fact_id,
                                    "payload": {"original": confidence, "adjusted": 0.75}
                                },
                                headers=session._headers()
                            )
                            confidence = 0.75

                        if justifications:
                            requests.post(f"{session.base_url}/facts", json={"id": fact_id, "is_root": False, "justification_sets": justifications, "payload": payload}, headers=session._headers())
                        else:
                            requests.post(f"{session.base_url}/facts", json={"id": fact_id, "is_root": True, "manual_status": float(confidence), "payload": payload}, headers=session._headers())
                    except Exception:
                        pass

        return response

class AsyncOpenAI(BaseAsyncOpenAI):
    """
    An asynchronous drop-in replacement for the OpenAI client that 
    automatically injects Velarix context and extracts facts.
    """
    def __init__(self, *args, velarix_base_url: Optional[str] = None, velarix_session_id: Optional[str] = None, **kwargs):
        super().__init__(*args, **kwargs)
        base_url = velarix_base_url or os.getenv("VELARIX_BASE_URL", "http://localhost:8080")
        self.velarix_client = AsyncVelarixClient(base_url=base_url)
        self.velarix_session_id = velarix_session_id

    @property
    def chat(self):
        return VelarixAsyncChat(self)

class VelarixAsyncChat:
    def __init__(self, client: AsyncOpenAI):
        self.client = client

    @property
    def completions(self):
        return VelarixAsyncCompletions(self.client)

class VelarixAsyncCompletions:
    def __init__(self, client: AsyncOpenAI):
        self.client = client
        self._base_completions = super(AsyncOpenAI, client).chat.completions

    async def create(self, *args, **kwargs):
        session_id = kwargs.pop("velarix_session_id", self.client.velarix_session_id)
        if not session_id:
            return await self._base_completions.create(*args, **kwargs)

        session = self.client.velarix_client.session(session_id)

        messages = kwargs.get("messages", [])
        context_markdown = await session.get_slice(format="markdown")
        
        system_instruction = (
            "\n\n## VELARIX EPISTEMIC PROTOCOL\n"
            "You are equipped with a memory layer (Velarix). Below are the current justified beliefs in this session. "
            "Use the 'record_observation' tool whenever you derive, infer, or assert a new fact that should be "
            "remembered. If your observation depends on any current beliefs, use their exact IDs (e.g., 'fact_123') "
            "in the 'justifications' field. Use an OR-of-ANDs format: [[id1, id2], [id3]].\n\n"
            "## CURRENT BELIEFS (Velarix)\n"
            f"{context_markdown}\n"
            "---\n"
        )

        system_msg = next((m for m in messages if m["role"] == "system"), None)
        if system_msg:
            system_msg["content"] = str(system_msg["content"]) + system_instruction
        else:
            messages.insert(0, {"role": "system", "content": "You are a helpful assistant." + system_instruction})

        tools = kwargs.get("tools", [])
        observation_tool = {
            "type": "function",
            "function": {
                "name": "record_observation",
                "description": "Persist a new justified belief, observation, or derived plan into long-term memory.",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "id": {"type": "string", "description": "A unique, slugified identifier."},
                        "payload": {"type": "object", "description": "The JSON data associated with this fact."},
                        "justifications": {
                            "type": "array",
                            "items": {"type": "array", "items": {"type": "string"}},
                            "description": "List of lists of Fact IDs that justify this observation."
                        },
                        "confidence": {"type": "number", "description": "Your confidence (0.0 to 1.0)."}
                    },
                    "required": ["id", "payload"]
                }
            }
        }
        tools.append(observation_tool)
        kwargs["tools"] = tools
        if "tool_choice" not in kwargs:
            kwargs["tool_choice"] = "auto"

        response = await self._base_completions.create(*args, **kwargs)

        choice = response.choices[0]
        if choice.message.tool_calls:
            for tool_call in choice.message.tool_calls:
                if tool_call.function.name == "record_observation":
                    try:
                        args = json.loads(tool_call.function.arguments)
                        fact_id = args.get("id")
                        payload = args.get("payload") or {}
                        justifications = args.get("justifications")
                        confidence = args.get("confidence", 1.0)

                        payload["_provenance"] = {
                            "source": "openai_interceptor_async",
                            "model": response.model,
                            "timestamp": response.created,
                            "tool_call_id": tool_call.id
                        }

                        is_root = not justifications or len(justifications) == 0
                        if is_root and confidence > 0.9:
                            async with httpx.AsyncClient() as http_client:
                                await http_client.post(
                                    f"{session.base_url}/history", 
                                    json={
                                        "type": "confidence_adjusted",
                                        "session_id": session_id,
                                        "fact_id": fact_id,
                                        "payload": {"original": confidence, "adjusted": 0.75}
                                    },
                                    headers=session._headers()
                                )
                            confidence = 0.75

                        if justifications:
                            async with httpx.AsyncClient() as http_client:
                                await http_client.post(f"{session.base_url}/facts", json={"id": fact_id, "is_root": False, "justification_sets": justifications, "payload": payload}, headers=session._headers())
                        else:
                            async with httpx.AsyncClient() as http_client:
                                await http_client.post(f"{session.base_url}/facts", json={"id": fact_id, "is_root": True, "manual_status": float(confidence), "payload": payload}, headers=session._headers())
                    except Exception:
                        pass

        return response

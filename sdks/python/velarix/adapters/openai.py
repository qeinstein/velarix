import json
import requests
from typing import Optional, List, Dict, Any, Union
from openai import OpenAI as BaseOpenAI
from velarix.client import VelarixClient

class OpenAI(BaseOpenAI):
    """
    A drop-in replacement for the OpenAI client that automatically 
    injects Velarix context and extracts facts.
    """
    def __init__(self, *args, velarix_base_url: str = "http://localhost:8080", velarix_session_id: Optional[str] = None, **kwargs):
        super().__init__(*args, **kwargs)
        self.velarix_client = VelarixClient(base_url=velarix_base_url)
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
        # 1. Check for session ID
        session_id = kwargs.pop("velarix_session_id", self.client.velarix_session_id)
        if not session_id:
            return self._base_completions.create(*args, **kwargs)

        session = self.client.velarix_client.session(session_id)

        # 2. Inject Context Slice and System Instructions
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

        # 3. Enhanced Tool Definition (JSON Schema)
        tools = kwargs.get("tools", [])
        observation_tool = {
            "type": "function",
            "function": {
                "name": "record_observation",
                "description": "Persist a new justified belief, observation, or derived plan into long-term memory.",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "id": {
                            "type": "string", 
                            "description": "A unique, slugified identifier (e.g., 'user_prefers_python_3_11')."
                        },
                        "payload": {
                            "type": "object", 
                            "description": "The JSON data associated with this fact."
                        },
                        "justifications": {
                            "type": "array",
                            "items": {
                                "type": "array",
                                "items": {"type": "string"}
                            },
                            "description": "List of lists of Fact IDs that justify this observation (OR-of-ANDs logic)."
                        },
                        "confidence": {
                            "type": "number",
                            "description": "Your confidence in this fact (0.0 to 1.0). Be honest: unsupported roots should rarely exceed 0.8."
                        }
                    },
                    "required": ["id", "payload"]
                }
            }
        }
        tools.append(observation_tool)
        kwargs["tools"] = tools
        if "tool_choice" not in kwargs:
            kwargs["tool_choice"] = "auto"

        # 4. Execute LLM Call
        response = self._base_completions.create(*args, **kwargs)

        # 5. Parallel Extraction & Interception
        choice = response.choices[0]
        if choice.message.tool_calls:
            for tool_call in choice.message.tool_calls:
                if tool_call.function.name == "record_observation":
                    try:
                        args = json.loads(tool_call.function.arguments)
                        fact_id = args.get("id")
                        payload = args.get("payload") or {}
                        justifications = args.get("justifications")
                        confidence = args.get("confidence", 1.0) # Default to 1.0 if not provided

                        # Tool-Call Provenance Injection
                        payload["_provenance"] = {
                            "source": "openai_interceptor",
                            "model": response.model,
                            "timestamp": response.created,
                            "tool_call_id": tool_call.id
                        }

                        # Overconfidence Sanity Check
                        is_root = not justifications or len(justifications) == 0
                        if is_root and confidence > 0.9:
                            # Log the adjustment for auditability
                            try:
                                session.client.headers.update({"X-Velarix-Event": "confidence_adjusted"})
                            except AttributeError:
                                # Fallback for mock objects or different client structures
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
                            requests.post(f"{session.base_url}/facts", json={"ID": fact_id, "IsRoot": False, "justification_sets": justifications, "payload": payload or {}}, headers=session._headers())
                        else:
                            # Map extracted confidence to ManualStatus for root facts
                            data = {"ID": fact_id, "IsRoot": True, "ManualStatus": float(confidence), "payload": payload or {}}
                            requests.post(f"{session.base_url}/facts", json=data, headers=session._headers())
                    except Exception as e:
                        # Log but do not block the assistant's response
                        print(f"[Velarix Interceptor Error] Failed to extract fact: {e}")

        return response

import os
from typing import Any, Dict, List, Optional

from openai import AsyncOpenAI as BaseAsyncOpenAI
from openai import OpenAI as BaseOpenAI

from velarix.client import AsyncVelarixClient, VelarixClient
from velarix.runtime import AsyncVelarixChatRuntime, VelarixChatRuntime


def _verification_feedback_text(verification: Dict[str, Any]) -> str:
    consistency = verification.get("consistency") or {}
    audits = verification.get("audits") or []
    parts: List[str] = []
    if consistency.get("issue_count"):
        parts.append(f"Consistency issues: {consistency.get('issues', [])}")
    failing_audits = [audit for audit in audits if not audit.get("valid", True)]
    if failing_audits:
        parts.append(f"Reasoning audit failures: {failing_audits}")
    joined = "\n".join(parts) if parts else "Verification found issues."
    return (
        "Velarix verification failed for the previous draft.\n"
        f"{joined}\n"
        "Revise the answer. Use semantic retrieval if needed, record a corrected reasoning chain, "
        "and retract contradicted earlier beliefs before finalizing."
    )


class OpenAI(BaseOpenAI):
    """
    Backward-compatible OpenAI client wrapper.

    The adapter surface remains a drop-in replacement, but all Velarix logic
    now lives in the shared chat runtime so future providers can reuse it.
    """

    def __init__(
        self,
        *args,
        velarix_base_url: Optional[str] = None,
        velarix_session_id: Optional[str] = None,
        velarix_strict: bool = True,
        velarix_verify_rounds: int = 1,
        **kwargs
    ):
        super().__init__(*args, **kwargs)
        base_url = velarix_base_url or os.getenv("VELARIX_BASE_URL", "http://localhost:8080")
        self.velarix_client = VelarixClient(base_url=base_url)
        self.velarix_session_id = velarix_session_id
        self.velarix_strict = velarix_strict
        self.velarix_verify_rounds = max(0, int(velarix_verify_rounds))

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

        verify_rounds = max(0, int(kwargs.pop("velarix_verify_rounds", self.client.velarix_verify_rounds)))
        auto_verify = bool(kwargs.pop("velarix_auto_verify", True))
        session = self.client.velarix_client.session(session_id)
        runtime = VelarixChatRuntime(
            session=session,
            source="openai_adapter",
            strict=self.client.velarix_strict,
        )
        base_params = dict(kwargs)
        base_messages = list(base_params.get("messages") or [])
        feedback_messages: List[Dict[str, str]] = []

        for round_idx in range(verify_rounds + 1):
            request = dict(base_params)
            request["messages"] = base_messages + feedback_messages
            params = runtime.prepare_params(request)
            response = self._base_completions.create(*args, **params)
            runtime.process_response(response)
            if not auto_verify:
                return response
            verification = runtime.verify_recent_reasoning(auto_retract=True)
            if not verification.get("has_issues"):
                return response
            if round_idx >= verify_rounds:
                return response
            feedback_messages.append({"role": "user", "content": _verification_feedback_text(verification)})

        return response


class AsyncOpenAI(BaseAsyncOpenAI):
    """
    Backward-compatible async OpenAI client wrapper built on the shared runtime.
    """

    def __init__(
        self,
        *args,
        velarix_base_url: Optional[str] = None,
        velarix_session_id: Optional[str] = None,
        velarix_strict: bool = True,
        velarix_verify_rounds: int = 1,
        **kwargs
    ):
        super().__init__(*args, **kwargs)
        base_url = velarix_base_url or os.getenv("VELARIX_BASE_URL", "http://localhost:8080")
        self.velarix_client = AsyncVelarixClient(base_url=base_url)
        self.velarix_session_id = velarix_session_id
        self.velarix_strict = velarix_strict
        self.velarix_verify_rounds = max(0, int(velarix_verify_rounds))

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

        verify_rounds = max(0, int(kwargs.pop("velarix_verify_rounds", self.client.velarix_verify_rounds)))
        auto_verify = bool(kwargs.pop("velarix_auto_verify", True))
        session = self.client.velarix_client.session(session_id)
        runtime = AsyncVelarixChatRuntime(
            session=session,
            source="openai_adapter_async",
            strict=self.client.velarix_strict,
        )
        base_params = dict(kwargs)
        base_messages = list(base_params.get("messages") or [])
        feedback_messages: List[Dict[str, str]] = []

        for round_idx in range(verify_rounds + 1):
            request = dict(base_params)
            request["messages"] = base_messages + feedback_messages
            params = await runtime.prepare_params(request)
            response = await self._base_completions.create(*args, **params)
            await runtime.process_response(response)
            if not auto_verify:
                return response
            verification = await runtime.verify_recent_reasoning(auto_retract=True)
            if not verification.get("has_issues"):
                return response
            if round_idx >= verify_rounds:
                return response
            feedback_messages.append({"role": "user", "content": _verification_feedback_text(verification)})

        return response

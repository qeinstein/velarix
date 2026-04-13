import os
from typing import Any, Dict, List, Optional, Tuple

from openai import AsyncOpenAI as BaseAsyncOpenAI
from openai import OpenAI as BaseOpenAI

from velarix.client import AsyncVelarixClient, VelarixClient, VelarixRuntimeError
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


def _resolve_velarix_settings(
    velarix_base_url: Optional[str] = None,
    velarix_api_key: Optional[str] = None,
) -> Tuple[str, str]:
    base_url = (velarix_base_url or os.getenv("VELARIX_BASE_URL", "http://localhost:8080")).rstrip("/")
    api_key = velarix_api_key or os.getenv("VELARIX_API_KEY")
    if not api_key:
        raise VelarixRuntimeError(
            "Velarix API key is required for the OpenAI adapter. "
            "Pass velarix_api_key=... or set VELARIX_API_KEY."
        )
    return base_url, api_key


class OpenAI(BaseOpenAI):
    """
    Backward-compatible OpenAI client wrapper.

    The adapter surface remains a drop-in replacement, but all Velarix logic
    now lives in the shared chat runtime so future providers can reuse it.
    """

    def __init__(
        self,
        *args,
        velarix_api_key: Optional[str] = None,
        velarix_base_url: Optional[str] = None,
        velarix_session_id: Optional[str] = None,
        velarix_strict: bool = True,
        velarix_verify_rounds: int = 1,
        **kwargs
    ):
        resolved_base_url, resolved_api_key = _resolve_velarix_settings(velarix_base_url, velarix_api_key)
        super().__init__(*args, **kwargs)
        self.velarix_base_url = resolved_base_url
        self.velarix_api_key = resolved_api_key
        self.velarix_client = VelarixClient(base_url=resolved_base_url, api_key=resolved_api_key)
        self.velarix_session_id = velarix_session_id
        self.velarix_strict = velarix_strict
        self.velarix_verify_rounds = max(0, int(velarix_verify_rounds))

    @property
    def chat(self):
        """Return the Velarix-aware chat adapter for this client."""
        return VelarixChat(
            self,
            velarix_api_key=self.velarix_api_key,
            velarix_base_url=self.velarix_base_url,
        )


class VelarixChat:
    """Expose a Velarix-backed `chat.completions` surface on the sync OpenAI client."""

    def __init__(
        self,
        client: OpenAI,
        velarix_api_key: Optional[str] = None,
        velarix_base_url: Optional[str] = None,
    ):
        self.client = client
        resolved_base_url, resolved_api_key = _resolve_velarix_settings(
            velarix_base_url or getattr(client, "velarix_base_url", None),
            velarix_api_key or getattr(client, "velarix_api_key", None),
        )
        self.velarix_base_url = resolved_base_url
        self.velarix_api_key = resolved_api_key
        if (
            getattr(client, "velarix_base_url", None) == resolved_base_url
            and getattr(client, "velarix_api_key", None) == resolved_api_key
        ):
            self.velarix_client = client.velarix_client
        else:
            self.velarix_client = VelarixClient(base_url=resolved_base_url, api_key=resolved_api_key)

    @property
    def completions(self):
        """Return the Velarix-aware chat completions adapter."""
        return VelarixCompletions(
            self.client,
            velarix_api_key=self.velarix_api_key,
            velarix_base_url=self.velarix_base_url,
        )


class VelarixCompletions:
    """Wrap sync chat completions with Velarix context injection and verification."""

    def __init__(
        self,
        client: OpenAI,
        velarix_api_key: Optional[str] = None,
        velarix_base_url: Optional[str] = None,
    ):
        self.client = client
        self._base_completions = super(OpenAI, client).chat.completions
        resolved_base_url, resolved_api_key = _resolve_velarix_settings(
            velarix_base_url or getattr(client, "velarix_base_url", None),
            velarix_api_key or getattr(client, "velarix_api_key", None),
        )
        self.velarix_base_url = resolved_base_url
        self.velarix_api_key = resolved_api_key
        if (
            getattr(client, "velarix_base_url", None) == resolved_base_url
            and getattr(client, "velarix_api_key", None) == resolved_api_key
        ):
            self.velarix_client = client.velarix_client
        else:
            self.velarix_client = VelarixClient(base_url=resolved_base_url, api_key=resolved_api_key)

    def create(self, *args, **kwargs):
        """Call `chat.completions.create` and optionally persist and verify Velarix reasoning."""
        session_id = kwargs.pop("velarix_session_id", self.client.velarix_session_id)
        if not session_id:
            return self._base_completions.create(*args, **kwargs)

        verify_rounds = max(0, int(kwargs.pop("velarix_verify_rounds", self.client.velarix_verify_rounds)))
        auto_verify = bool(kwargs.pop("velarix_auto_verify", True))
        session = self.velarix_client.session(session_id)
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
        velarix_api_key: Optional[str] = None,
        velarix_base_url: Optional[str] = None,
        velarix_session_id: Optional[str] = None,
        velarix_strict: bool = True,
        velarix_verify_rounds: int = 1,
        **kwargs
    ):
        resolved_base_url, resolved_api_key = _resolve_velarix_settings(velarix_base_url, velarix_api_key)
        super().__init__(*args, **kwargs)
        self.velarix_base_url = resolved_base_url
        self.velarix_api_key = resolved_api_key
        self.velarix_client = AsyncVelarixClient(base_url=resolved_base_url, api_key=resolved_api_key)
        self.velarix_session_id = velarix_session_id
        self.velarix_strict = velarix_strict
        self.velarix_verify_rounds = max(0, int(velarix_verify_rounds))

    @property
    def chat(self):
        """Return the Velarix-aware chat adapter for this async client."""
        return VelarixAsyncChat(
            self,
            velarix_api_key=self.velarix_api_key,
            velarix_base_url=self.velarix_base_url,
        )


class VelarixAsyncChat:
    """Expose a Velarix-backed `chat.completions` surface on the async OpenAI client."""

    def __init__(
        self,
        client: AsyncOpenAI,
        velarix_api_key: Optional[str] = None,
        velarix_base_url: Optional[str] = None,
    ):
        self.client = client
        resolved_base_url, resolved_api_key = _resolve_velarix_settings(
            velarix_base_url or getattr(client, "velarix_base_url", None),
            velarix_api_key or getattr(client, "velarix_api_key", None),
        )
        self.velarix_base_url = resolved_base_url
        self.velarix_api_key = resolved_api_key
        if (
            getattr(client, "velarix_base_url", None) == resolved_base_url
            and getattr(client, "velarix_api_key", None) == resolved_api_key
        ):
            self.velarix_client = client.velarix_client
        else:
            self.velarix_client = AsyncVelarixClient(base_url=resolved_base_url, api_key=resolved_api_key)

    @property
    def completions(self):
        """Return the async Velarix-aware chat completions adapter."""
        return VelarixAsyncCompletions(
            self.client,
            velarix_api_key=self.velarix_api_key,
            velarix_base_url=self.velarix_base_url,
        )


class VelarixAsyncCompletions:
    """Wrap async chat completions with Velarix context injection and verification."""

    def __init__(
        self,
        client: AsyncOpenAI,
        velarix_api_key: Optional[str] = None,
        velarix_base_url: Optional[str] = None,
    ):
        self.client = client
        self._base_completions = super(AsyncOpenAI, client).chat.completions
        resolved_base_url, resolved_api_key = _resolve_velarix_settings(
            velarix_base_url or getattr(client, "velarix_base_url", None),
            velarix_api_key or getattr(client, "velarix_api_key", None),
        )
        self.velarix_base_url = resolved_base_url
        self.velarix_api_key = resolved_api_key
        if (
            getattr(client, "velarix_base_url", None) == resolved_base_url
            and getattr(client, "velarix_api_key", None) == resolved_api_key
        ):
            self.velarix_client = client.velarix_client
        else:
            self.velarix_client = AsyncVelarixClient(base_url=resolved_base_url, api_key=resolved_api_key)

    async def create(self, *args, **kwargs):
        """Call `chat.completions.create` and optionally persist and verify Velarix reasoning."""
        session_id = kwargs.pop("velarix_session_id", self.client.velarix_session_id)
        if not session_id:
            return await self._base_completions.create(*args, **kwargs)

        verify_rounds = max(0, int(kwargs.pop("velarix_verify_rounds", self.client.velarix_verify_rounds)))
        auto_verify = bool(kwargs.pop("velarix_auto_verify", True))
        session = self.velarix_client.session(session_id)
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

import os
from typing import Optional

from openai import AsyncOpenAI as BaseAsyncOpenAI
from openai import OpenAI as BaseOpenAI

from velarix.client import AsyncVelarixClient, VelarixClient
from velarix.runtime import AsyncVelarixChatRuntime, VelarixChatRuntime


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
        **kwargs
    ):
        super().__init__(*args, **kwargs)
        base_url = velarix_base_url or os.getenv("VELARIX_BASE_URL", "http://localhost:8080")
        self.velarix_client = VelarixClient(base_url=base_url)
        self.velarix_session_id = velarix_session_id
        self.velarix_strict = velarix_strict

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
        runtime = VelarixChatRuntime(
            session=session,
            source="openai_adapter",
            strict=self.client.velarix_strict,
        )
        params = runtime.prepare_params(kwargs)
        response = self._base_completions.create(*args, **params)
        return runtime.process_response(response)


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
        **kwargs
    ):
        super().__init__(*args, **kwargs)
        base_url = velarix_base_url or os.getenv("VELARIX_BASE_URL", "http://localhost:8080")
        self.velarix_client = AsyncVelarixClient(base_url=base_url)
        self.velarix_session_id = velarix_session_id
        self.velarix_strict = velarix_strict

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
        runtime = AsyncVelarixChatRuntime(
            session=session,
            source="openai_adapter_async",
            strict=self.client.velarix_strict,
        )
        params = await runtime.prepare_params(kwargs)
        response = await self._base_completions.create(*args, **params)
        return await runtime.process_response(response)

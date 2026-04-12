import asyncio
import os
import sys
import time
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
import requests

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from velarix.client import AsyncVelarixClient, VelarixClient


_CACHE_KEY = ("json", 50, "", "", None, False, 0)


def test_sync_extract_and_assert_posts_and_clears_cache():
    client = VelarixClient(base_url="http://localhost:8080", api_key="test_key")
    session = client.session("sync_extract_session")
    session._slice_cache[_CACHE_KEY] = (time.time(), [{"id": "cached_fact"}])

    response = MagicMock()
    response.raise_for_status.return_value = None
    response.json.return_value = {"asserted_count": 2, "facts": [{"id": "fact_1"}]}

    with patch.object(client, "_request", return_value=response) as mock_request:
        result = session.extract_and_assert(
            "The user is approved.",
            session_context="Prior approval context",
            auto_retract_contradictions=True,
        )

    assert result == {"asserted_count": 2, "facts": [{"id": "fact_1"}]}
    mock_request.assert_called_once_with(
        "POST",
        "http://localhost:8080/v1/s/sync_extract_session/extract-and-assert",
        json={
            "llm_output": "The user is approved.",
            "session_context": "Prior approval context",
            "auto_retract_contradictions": True,
        },
        headers=client.headers,
    )
    assert session._slice_cache == {}


def test_sync_extract_and_assert_raises_http_error():
    client = VelarixClient(base_url="http://localhost:8080", api_key="test_key")
    session = client.session("sync_extract_error_session")
    session._slice_cache[_CACHE_KEY] = (time.time(), [{"id": "cached_fact"}])

    response = MagicMock()
    response.raise_for_status.side_effect = requests.HTTPError("bad extract response")

    with patch.object(client, "_request", return_value=response):
        with pytest.raises(requests.HTTPError):
            session.extract_and_assert("Bad output")

    assert _CACHE_KEY in session._slice_cache


def test_async_extract_and_assert_posts_and_clears_cache():
    async def run() -> None:
        async with AsyncVelarixClient(base_url="http://localhost:8080", api_key="test_key") as client:
            session = client.session("async_extract_session")
            session._slice_cache[_CACHE_KEY] = (time.time(), [{"id": "cached_fact"}])

            response = MagicMock()
            response.raise_for_status.return_value = None
            response.json.return_value = {"asserted_count": 1, "facts": [{"id": "fact_async"}]}

            with patch.object(client, "_request", new=AsyncMock(return_value=response)) as mock_request:
                result = await session.extract_and_assert(
                    "Async approval output",
                    session_context="Async prior context",
                    auto_retract_contradictions=False,
                )

            assert result == {"asserted_count": 1, "facts": [{"id": "fact_async"}]}
            mock_request.assert_awaited_once_with(
                "POST",
                "http://localhost:8080/v1/s/async_extract_session/extract-and-assert",
                json={
                    "llm_output": "Async approval output",
                    "session_context": "Async prior context",
                    "auto_retract_contradictions": False,
                },
                headers=client.headers,
            )
            assert session._slice_cache == {}

    asyncio.run(run())


def test_async_extract_and_assert_raises_http_error():
    async def run() -> None:
        async with AsyncVelarixClient(base_url="http://localhost:8080", api_key="test_key") as client:
            session = client.session("async_extract_error_session")
            session._slice_cache[_CACHE_KEY] = (time.time(), [{"id": "cached_fact"}])

            response = MagicMock()
            response.raise_for_status.side_effect = requests.HTTPError("bad async extract response")

            with patch.object(client, "_request", new=AsyncMock(return_value=response)):
                with pytest.raises(requests.HTTPError):
                    await session.extract_and_assert("Bad async output")

            assert _CACHE_KEY in session._slice_cache

    asyncio.run(run())

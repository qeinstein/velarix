"""Tests for typed error handling, retry jitter, and plan-awareness."""
import threading
import time
from unittest.mock import MagicMock, patch

import pytest
import requests

from velarix.client import (
    AuthError,
    PlanLimitError,
    RateLimitError,
    VelarixClient,
    VelarixError,
    _raise_for_status,
)


# ---------------------------------------------------------------------------
# _raise_for_status
# ---------------------------------------------------------------------------

def _mock_response(status: int, json_body=None, headers=None, text=""):
    resp = MagicMock()
    resp.status_code = status
    resp.headers = headers or {}
    resp.text = text
    if json_body is not None:
        resp.json.return_value = json_body
    else:
        resp.json.side_effect = ValueError("no body")
    return resp


def test_raise_for_status_passes_2xx():
    _raise_for_status(_mock_response(200))
    _raise_for_status(_mock_response(201))


def test_raise_for_status_401_raises_auth_error():
    with pytest.raises(AuthError) as exc_info:
        _raise_for_status(_mock_response(401))
    assert exc_info.value.status_code == 401
    assert isinstance(exc_info.value, VelarixError)


def test_raise_for_status_402_raises_plan_limit_error():
    resp = _mock_response(402, json_body={"plan": "free", "limit": 50})
    with pytest.raises(PlanLimitError) as exc_info:
        _raise_for_status(resp)
    err = exc_info.value
    assert err.status_code == 402
    assert err.plan == "free"
    assert err.limit == 50
    assert isinstance(err, VelarixError)


def test_raise_for_status_402_no_body():
    resp = _mock_response(402, text="plan limit reached")
    with pytest.raises(PlanLimitError) as exc_info:
        _raise_for_status(resp)
    assert exc_info.value.plan == ""
    assert exc_info.value.limit is None


def test_raise_for_status_429_raises_rate_limit_error():
    resp = _mock_response(429, headers={"Retry-After": "3.5"})
    with pytest.raises(RateLimitError) as exc_info:
        _raise_for_status(resp)
    err = exc_info.value
    assert err.status_code == 429
    assert err.retry_after == 3.5


def test_raise_for_status_429_no_retry_after():
    resp = _mock_response(429)
    with pytest.raises(RateLimitError) as exc_info:
        _raise_for_status(resp)
    assert exc_info.value.retry_after is None


def test_raise_for_status_500_uses_requests_raise():
    resp = MagicMock()
    resp.status_code = 500
    resp.text = "internal error"
    resp.headers = {}
    resp.raise_for_status.side_effect = requests.HTTPError("500")
    with pytest.raises(requests.HTTPError):
        _raise_for_status(resp)


# ---------------------------------------------------------------------------
# PlanLimitError propagates from client methods
# ---------------------------------------------------------------------------

def _client_with_mock(status: int, json_body=None, headers=None):
    """Return a VelarixClient whose HTTP layer always returns the given status."""
    client = VelarixClient(base_url="http://localhost:9999", api_key="test-key", max_retries=0)
    mock_resp = _mock_response(status, json_body=json_body, headers=headers or {})
    mock_resp.raise_for_status.side_effect = requests.HTTPError(str(status))
    client._http.request = MagicMock(return_value=mock_resp)
    return client


def test_get_billing_401_raises_auth_error():
    client = _client_with_mock(401)
    with pytest.raises(AuthError):
        client.get_billing()


def test_create_session_402_raises_plan_limit_error():
    client = _client_with_mock(402, json_body={"plan": "free", "limit": 50, "error": "limit"})
    with pytest.raises(PlanLimitError) as exc_info:
        client.session("s1").observe("fact1", {"value": 1})
    assert exc_info.value.plan == "free"


def test_get_me_429_raises_rate_limit_error():
    client = _client_with_mock(429, headers={"Retry-After": "2"})
    with pytest.raises(RateLimitError) as exc_info:
        client.get_me()
    assert exc_info.value.retry_after == 2.0


# ---------------------------------------------------------------------------
# plan property
# ---------------------------------------------------------------------------

def test_plan_property_unknown_before_fetch():
    client = VelarixClient(api_key="test", max_retries=0)
    assert client.plan == "unknown"


def test_plan_property_cached_after_get_billing():
    client = _client_with_mock(200, json_body={"plan": "pro", "status": "active"})
    client.get_billing()
    assert client.plan == "pro"


# ---------------------------------------------------------------------------
# Retry jitter — ensure delay has variance
# ---------------------------------------------------------------------------

def test_retry_jitter_produces_variance():
    """Delays should vary across retries due to jitter, not be identical."""
    delays = []
    real_sleep = time.sleep

    def capture_sleep(s):
        delays.append(s)

    client = VelarixClient(base_url="http://localhost:9999", api_key="k", max_retries=3, retry_backoff_base=1.0)
    resp_503 = _mock_response(503)
    resp_503.raise_for_status.side_effect = requests.HTTPError("503")
    resp_200 = _mock_response(200, json_body={"plan": "free"})

    call_count = 0

    def side_effect(*args, **kwargs):
        nonlocal call_count
        call_count += 1
        return resp_503 if call_count <= 3 else resp_200

    client._http.request = MagicMock(side_effect=side_effect)

    with patch("velarix.client.time.sleep", side_effect=capture_sleep):
        client.get_billing()

    assert len(delays) >= 2
    # With jitter the delays should not all be exactly equal
    assert not all(d == delays[0] for d in delays), "expected jitter to produce varying delays"


# ---------------------------------------------------------------------------
# Thread safety — _slice_cache under concurrent access
# ---------------------------------------------------------------------------

def test_slice_cache_thread_safety():
    """Concurrent reads and clears must not raise RuntimeError."""
    client = VelarixClient(base_url="http://localhost:9999", api_key="k", cache_ttl=60, max_retries=0)
    session = client.session("s1")

    # Populate cache manually
    for i in range(50):
        with session._cache_lock:
            session._slice_cache[(str(i), i)] = (time.time(), {"data": i})

    errors = []

    def reader():
        for _ in range(200):
            try:
                with session._cache_lock:
                    _ = dict(session._slice_cache)
            except Exception as e:
                errors.append(e)

    def clearer():
        for _ in range(100):
            try:
                session._clear_cache()
            except Exception as e:
                errors.append(e)

    threads = [threading.Thread(target=reader) for _ in range(4)] + [threading.Thread(target=clearer) for _ in range(2)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()

    assert errors == [], f"Thread safety errors: {errors}"

import json
from collections.abc import Callable, Iterator

import httpx2
import pytest
from fastapi.testclient import TestClient

from app.main import app, get_ollama
from app.ollama import OllamaClient

Handler = Callable[[httpx2.Request], httpx2.Response]

client = TestClient(app)


@pytest.fixture
def fake_ollama() -> Iterator[Callable[[Handler], None]]:
    """Route the app's Ollama calls to `handler` instead of a real server."""

    def install(handler: Handler) -> None:
        http = httpx2.AsyncClient(
            base_url="http://ollama.test", transport=httpx2.MockTransport(handler)
        )
        ollama = OllamaClient(http, model="test-model")
        app.dependency_overrides[get_ollama] = lambda: ollama

    yield install
    app.dependency_overrides.clear()


def test_generate_returns_llm_text(fake_ollama: Callable[[Handler], None]) -> None:
    sent: list[dict[str, object]] = []

    def handler(request: httpx2.Request) -> httpx2.Response:
        assert request.url.path == "/api/generate"
        sent.append(json.loads(request.content))
        reply = {"model": "test-model", "response": "Hi!", "done": True}
        return httpx2.Response(200, json=reply)

    fake_ollama(handler)

    response = client.post("/generate", json={"prompt": "Say hi"})

    assert response.status_code == 200
    assert response.json() == {"text": "Hi!", "model": "test-model"}
    assert sent == [
        {"model": "test-model", "prompt": "Say hi", "stream": False, "think": False}
    ]


def _status_500(request: httpx2.Request) -> httpx2.Response:
    return httpx2.Response(500, json={"error": "model not found"})


def _not_json(request: httpx2.Request) -> httpx2.Response:
    return httpx2.Response(200, text="not json")


def _connect_error(request: httpx2.Request) -> httpx2.Response:
    raise httpx2.ConnectError("connection refused", request=request)


def _read_timeout(request: httpx2.Request) -> httpx2.Response:
    raise httpx2.ReadTimeout("timed out", request=request)


@pytest.mark.parametrize(
    ("handler", "want_status"),
    [
        pytest.param(_status_500, 502, id="ollama returns 500"),
        pytest.param(_not_json, 502, id="ollama returns invalid body"),
        pytest.param(_connect_error, 502, id="ollama is down"),
        pytest.param(_read_timeout, 504, id="ollama times out"),
    ],
)
def test_generate_maps_ollama_errors(
    fake_ollama: Callable[[Handler], None], handler: Handler, want_status: int
) -> None:
    fake_ollama(handler)

    response = client.post("/generate", json={"prompt": "Say hi"})

    assert response.status_code == want_status


def test_generate_rejects_empty_prompt(fake_ollama: Callable[[Handler], None]) -> None:
    def handler(request: httpx2.Request) -> httpx2.Response:
        raise AssertionError("Ollama must not be called for an invalid request")

    fake_ollama(handler)

    response = client.post("/generate", json={"prompt": ""})

    assert response.status_code == 422

import json
from collections.abc import Callable, Iterator

import httpx2
import pytest
from fastapi.testclient import TestClient

from app.main import app, get_ollama
from app.ollama import OllamaClient
from app.schemas import EnrichResponse

Handler = Callable[[httpx2.Request], httpx2.Response]

client = TestClient(app)

VALID_CARD = {
    "translation": "выкатывать",
    "example": "We roll out the fix tonight.",
    "it_context": "Developers say it when a new version goes to production.",
}


@pytest.fixture
def fake_ollama() -> Iterator[Callable[[Handler], None]]:
    def install(handler: Handler) -> None:
        http = httpx2.AsyncClient(
            base_url="http://ollama.test", transport=httpx2.MockTransport(handler)
        )
        ollama = OllamaClient(http, model="test-model")
        app.dependency_overrides[get_ollama] = lambda: ollama

    yield install
    app.dependency_overrides.clear()


class ScriptedOllama:
    """Answers each call with the next text from `texts` and records the requests."""

    def __init__(self, texts: list[str]) -> None:
        self._texts = iter(texts)
        self.requests: list[dict[str, object]] = []

    def __call__(self, request: httpx2.Request) -> httpx2.Response:
        self.requests.append(json.loads(request.content))
        return httpx2.Response(200, json={"response": next(self._texts), "done": True})


def test_enrich_returns_card(fake_ollama: Callable[[Handler], None]) -> None:
    ollama = ScriptedOllama([json.dumps(VALID_CARD)])
    fake_ollama(ollama)

    response = client.post("/enrich", json={"phrase": "roll out"})

    assert response.status_code == 200
    assert response.json() == VALID_CARD
    assert len(ollama.requests) == 1
    sent = ollama.requests[0]
    assert "format" not in sent
    prompt = str(sent["prompt"])
    assert "roll out" in prompt
    assert json.dumps(EnrichResponse.model_json_schema()) in prompt


@pytest.mark.parametrize(
    "answer",
    [
        pytest.param(f"```json\n{json.dumps(VALID_CARD)}\n```", id="json fence"),
        pytest.param(f"```\n{json.dumps(VALID_CARD)}\n```", id="plain fence"),
        pytest.param(f"```JSON\n{json.dumps(VALID_CARD)}\n```", id="upper label"),
        pytest.param(f"``` json\n{json.dumps(VALID_CARD)}\n```", id="space label"),
        pytest.param(f"Here is the card:\n{json.dumps(VALID_CARD)}", id="text before"),
    ],
)
def test_enrich_accepts_json_in_wrapper(
    fake_ollama: Callable[[Handler], None], answer: str
) -> None:
    ollama = ScriptedOllama([answer])
    fake_ollama(ollama)

    response = client.post("/enrich", json={"phrase": "roll out"})

    assert response.status_code == 200
    assert response.json() == VALID_CARD
    assert len(ollama.requests) == 1


def test_enrich_retries_after_invalid_json(
    fake_ollama: Callable[[Handler], None],
) -> None:
    ollama = ScriptedOllama(["not json", json.dumps(VALID_CARD)])
    fake_ollama(ollama)

    response = client.post("/enrich", json={"phrase": "roll out"})

    assert response.status_code == 200
    assert response.json() == VALID_CARD
    assert len(ollama.requests) == 2


@pytest.mark.parametrize(
    "bad_answer",
    [
        pytest.param("not json", id="not json"),
        pytest.param(json.dumps({"translation": "x"}), id="missing fields"),
        pytest.param(json.dumps({**VALID_CARD, "example": ""}), id="empty field"),
        pytest.param(json.dumps({**VALID_CARD, "translation": " "}), id="blank field"),
    ],
)
def test_enrich_returns_502_after_two_invalid_answers(
    fake_ollama: Callable[[Handler], None],
    bad_answer: str,
    caplog: pytest.LogCaptureFixture,
) -> None:
    ollama = ScriptedOllama([bad_answer, bad_answer])
    fake_ollama(ollama)

    response = client.post("/enrich", json={"phrase": "roll out"})

    assert response.status_code == 502
    assert response.json() == {"detail": "LLM returned invalid JSON"}
    assert len(ollama.requests) == 2
    assert "enrich failed" in caplog.text


def test_enrich_does_not_retry_when_ollama_is_down(
    fake_ollama: Callable[[Handler], None],
) -> None:
    calls = 0

    def handler(request: httpx2.Request) -> httpx2.Response:
        nonlocal calls
        calls += 1
        raise httpx2.ConnectError("connection refused", request=request)

    fake_ollama(handler)

    response = client.post("/enrich", json={"phrase": "roll out"})

    assert response.status_code == 502
    assert calls == 1


@pytest.mark.parametrize(
    "phrase",
    [pytest.param("", id="empty"), pytest.param("   ", id="only spaces")],
)
def test_enrich_rejects_blank_phrase(
    fake_ollama: Callable[[Handler], None], phrase: str
) -> None:
    def handler(request: httpx2.Request) -> httpx2.Response:
        raise AssertionError("Ollama must not be called for an invalid request")

    fake_ollama(handler)

    response = client.post("/enrich", json={"phrase": phrase})

    assert response.status_code == 422

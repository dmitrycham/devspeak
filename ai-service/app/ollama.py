import httpx2
from pydantic import BaseModel, ValidationError


class OllamaError(Exception):
    """Ollama is unreachable or returned an answer we cannot use."""


class OllamaTimeoutError(Exception):
    """Ollama did not answer within the client timeout."""


class _GenerateReply(BaseModel):
    # Ollama sends more fields (timings, token counts); we only need the text.
    response: str


class OllamaClient:
    def __init__(self, http: httpx2.AsyncClient, model: str) -> None:
        # The app lifespan owns the shared HTTP client, so this class never closes it.
        self._http = http
        self.model = model

    async def generate(self, prompt: str) -> str:
        payload = {
            "model": self.model,
            "prompt": prompt,
            # One JSON answer instead of a stream of chunks.
            "stream": False,
            # qwen3.5 is a thinking model; thinking is slow and we never show it.
            "think": False,
        }
        try:
            response = await self._http.post("/api/generate", json=payload)
            response.raise_for_status()
        # TimeoutException is a subclass of HTTPError, so it must be caught first.
        except httpx2.TimeoutException as exc:
            raise OllamaTimeoutError("ollama timed out") from exc
        except httpx2.HTTPError as exc:
            raise OllamaError(f"ollama request failed: {exc}") from exc

        try:
            reply = _GenerateReply.model_validate_json(response.content)
        except ValidationError as exc:
            raise OllamaError("ollama returned an unexpected body") from exc
        return reply.response

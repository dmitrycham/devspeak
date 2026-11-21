import logging
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from typing import Annotated, Literal

import httpx2
from fastapi import Depends, FastAPI, HTTPException, Request
from pydantic import BaseModel

from app.config import Settings
from app.enrich import EnrichError, enrich
from app.ollama import OllamaClient, OllamaError, OllamaTimeoutError
from app.schemas import (
    EnrichRequest,
    EnrichResponse,
    GenerateRequest,
    GenerateResponse,
)

logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    settings = Settings()
    # One client for the whole app lifetime: its connection pool lets requests
    # reuse connections, and `async with` closes them on shutdown.
    async with httpx2.AsyncClient(
        base_url=settings.base_url, timeout=settings.timeout_seconds
    ) as http:
        app.state.ollama = OllamaClient(http, settings.model)
        yield


app = FastAPI(title="DevSpeak AI service", lifespan=lifespan)


class HealthResponse(BaseModel):
    status: Literal["ok"]


def get_ollama(request: Request) -> OllamaClient:
    # A dependency, so tests can swap in a client that talks to a fake Ollama.
    return request.app.state.ollama


@app.get("/health")
async def health() -> HealthResponse:
    return HealthResponse(status="ok")


@app.post("/generate")
async def generate(
    body: GenerateRequest,
    ollama: Annotated[OllamaClient, Depends(get_ollama)],
) -> GenerateResponse:
    try:
        text = await ollama.generate(body.prompt)
    # FastAPI does not log a raised HTTPException, so without these lines
    # the real cause (model not pulled, connection refused) would be lost.
    # OllamaTimeoutError is a subclass of OllamaError, so it must be caught first.
    except OllamaTimeoutError as exc:
        logger.warning("ollama call timed out: %s", exc)
        raise HTTPException(status_code=504, detail="LLM timed out") from exc
    except OllamaError as exc:
        logger.warning("ollama call failed: %s", exc)
        raise HTTPException(status_code=502, detail="LLM request failed") from exc
    return GenerateResponse(text=text, model=ollama.model)


@app.post("/enrich")
async def enrich_phrase(
    body: EnrichRequest,
    ollama: Annotated[OllamaClient, Depends(get_ollama)],
) -> EnrichResponse:
    try:
        return await enrich(ollama, body.phrase)
    except EnrichError as exc:
        logger.warning("enrich failed: %s", exc)
        raise HTTPException(
            status_code=502, detail="LLM returned invalid JSON"
        ) from exc
    except OllamaTimeoutError as exc:
        logger.warning("ollama call timed out: %s", exc)
        raise HTTPException(status_code=504, detail="LLM timed out") from exc
    except OllamaError as exc:
        logger.warning("ollama call failed: %s", exc)
        raise HTTPException(status_code=502, detail="LLM request failed") from exc

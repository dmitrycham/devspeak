from typing import Literal

from fastapi import FastAPI
from pydantic import BaseModel

app = FastAPI(title="DevSpeak AI service")


class HealthResponse(BaseModel):
    status: Literal["ok"]


@app.get("/health")
async def health() -> HealthResponse:
    return HealthResponse(status="ok")

from typing import Annotated

from pydantic import BaseModel, Field, StringConstraints

# Strip first, then check the length, so "   " is rejected like "".
# A blank card field is valid JSON but useless, so it must trigger the retry.
NonBlankStr = Annotated[str, StringConstraints(strip_whitespace=True, min_length=1)]


class GenerateRequest(BaseModel):
    prompt: str = Field(min_length=1)


class GenerateResponse(BaseModel):
    text: str
    model: str


class EnrichRequest(BaseModel):
    phrase: NonBlankStr


class EnrichResponse(BaseModel):
    translation: NonBlankStr
    example: NonBlankStr
    it_context: NonBlankStr

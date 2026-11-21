import json
import logging

from pydantic import ValidationError

from app.ollama import OllamaClient
from app.prompts import ENRICH_PROMPT
from app.schemas import EnrichResponse

logger = logging.getLogger(__name__)

MAX_ATTEMPTS = 2

# Ollama 0.40.16 answers 501 "structured output is unavailable" for qwen3.5:4b,
# even for format="json". So the schema goes into the prompt as text, and
# Pydantic validation with one retry catches the answers that do not match it.
_SCHEMA_TEXT = json.dumps(EnrichResponse.model_json_schema())


class EnrichError(Exception):
    """The LLM did not return a valid card after all attempts."""


def _extract_json_object(text: str) -> str:
    # Models wrap JSON in ``` fences with any label or add words around it.
    # The object is everything from the first "{" to the last "}".
    start, end = text.find("{"), text.rfind("}")
    if start == -1 or end < start:
        # Nothing to cut out; Pydantic reports the error and we retry.
        return text
    return text[start : end + 1]


async def enrich(ollama: OllamaClient, phrase: str) -> EnrichResponse:
    prompt = ENRICH_PROMPT.format(phrase=phrase, schema=_SCHEMA_TEXT)
    for attempt in range(1, MAX_ATTEMPTS + 1):
        text = await ollama.generate(prompt)
        try:
            return EnrichResponse.model_validate_json(_extract_json_object(text))
        # Only an invalid answer is retried: the model may be right next time.
        # Network errors and timeouts (OllamaError) are not caught here.
        except ValidationError as exc:
            logger.warning("enrich attempt %d gave invalid JSON: %s", attempt, exc)
    raise EnrichError(f"no valid JSON after {MAX_ATTEMPTS} attempts")

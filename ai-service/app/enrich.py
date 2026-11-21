import logging

from pydantic import ValidationError

from app.ollama import OllamaClient
from app.prompts import ENRICH_PROMPT
from app.schemas import EnrichResponse

logger = logging.getLogger(__name__)

MAX_ATTEMPTS = 2


class EnrichError(Exception):
    """The LLM did not return a valid card after all attempts."""


async def enrich(ollama: OllamaClient, phrase: str) -> EnrichResponse:
    prompt = ENRICH_PROMPT.format(phrase=phrase)
    schema = EnrichResponse.model_json_schema()
    for attempt in range(1, MAX_ATTEMPTS + 1):
        text = await ollama.generate(prompt, schema=schema)
        try:
            return EnrichResponse.model_validate_json(text)
        # Only an invalid answer is retried: the model may be right next time.
        # Network errors and timeouts (OllamaError) are not caught here.
        except ValidationError as exc:
            logger.warning("enrich attempt %d gave invalid JSON: %s", attempt, exc)
    raise EnrichError(f"no valid JSON after {MAX_ATTEMPTS} attempts")

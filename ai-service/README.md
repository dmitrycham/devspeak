# ai-service

Python part of DevSpeak. It calls a local LLM through [Ollama](https://ollama.com).

## Requirements

- [uv](https://docs.astral.sh/uv/). It installs Python 3.14 from `.python-version` if you do not have it.
- Ollama with the model pulled: `ollama pull qwen3.5:4b`.

## Settings

All settings have defaults for local development. Set them as environment variables to change them.

| Variable | Default | Meaning |
|---|---|---|
| `OLLAMA_BASE_URL` | `http://localhost:11434` | Ollama server |
| `OLLAMA_MODEL` | `qwen3.5:4b` | Model name |
| `OLLAMA_TIMEOUT_SECONDS` | `60` | Timeout for one Ollama call |

## Run

```sh
cd ai-service
uv sync
uv run uvicorn app.main:app --reload
```

Check it: `curl http://127.0.0.1:8000/health` returns `{"status":"ok"}`.

Ask the LLM:

```sh
curl -X POST http://127.0.0.1:8000/generate -H 'Content-Type: application/json' -d '{"prompt": "Explain a goroutine in one sentence."}'
```

It returns `{"text": "...", "model": "qwen3.5:4b"}`. Errors: `422` for an empty prompt, `502` if Ollama is down or fails, `504` if Ollama times out.

Enrich a phrase for a card:

```sh
curl -X POST http://127.0.0.1:8000/enrich -H 'Content-Type: application/json' -d '{"phrase": "roll out"}'
```

It returns `{"translation": "...", "example": "...", "it_context": "..."}`. The JSON schema comes from the Pydantic model `EnrichResponse` and is sent to Ollama in the `format` field. An invalid answer is retried once. Errors: `422` for an empty phrase, `502` if the answer is invalid twice or Ollama fails, `504` if Ollama times out.

## Tests and linter

```sh
cd ai-service
uv run pytest
uv run ruff check .
```

CI runs the same commands on every pull request (job `python` in `.github/workflows/ci.yml`).

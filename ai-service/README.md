# ai-service

Python part of DevSpeak. It will call the LLM for the interview trainer. For now it has only a health check.

## Requirements

- [uv](https://docs.astral.sh/uv/). It installs Python 3.14 from `.python-version` if you do not have it.

## Run

```sh
cd ai-service
uv sync
uv run uvicorn app.main:app --reload
```

Check it: `curl http://127.0.0.1:8000/health` returns `{"status":"ok"}`.

## Tests and linter

```sh
cd ai-service
uv run pytest
uv run ruff check .
```

CI runs the same commands on every pull request (job `python` in `.github/workflows/ci.yml`).

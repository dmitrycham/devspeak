# DevSpeak — project rules

General rules are in `~/.claude/CLAUDE.md`. This file is only about this project.

## Project
DevSpeak: English for developers. A smart vocabulary with spaced repetition and an AI interview trainer.
One repository, two services:
- `go-api/` — Go: REST API, Telegram bot, PostgreSQL, spaced repetition (SM-2).
- `ai-service/` — Python 3.14: async FastAPI, Pydantic, LLM calls (Ollama locally, cloud API later), structured output, tool calling, RAG with pgvector.
- `docker-compose.yml` — starts Go API, AI service, PostgreSQL (with pgvector) and Ollama.
- The services talk to each other over HTTP with JSON.

## Commands
- Go API: `cd go-api && go test ./... && golangci-lint run`
- AI service: `cd ai-service && uv run pytest && uv run ruff check .`
- Everything: `docker compose up --build`

## Current sprint
Sprint 1 (Nov 18–24): repository setup, CI, Go API for words + PostgreSQL in Docker.
(Updated every Wednesday after sprint planning.)

## Rules
- Local LLM: Ollama `qwen3.5:4b`.
- Go layers: handler → service → repository. Errors are wrapped with `%w`.
- Every new feature has tests.
- Secrets only in `.env`, never in Git.
- Branches: `feature/DEV-<n>-short-name`. Commits: Conventional Commits in English (`feat:`, `fix:`, `chore:`).
- Show a plan before big changes. Keep code simple: I must be able to explain every line.

## GitHub rules
- OK without asking: git status/diff/log, create a branch, commit and push to a feature branch, open a PR (gh pr create), read PRs and CI.
- Always ask first, even in auto mode: merge a PR (only after the team lead wrote "Approved"), delete a branch, any push to main, change repo settings, close a PR or issue.
- Never: force push to main, rewrite pushed history, git reset --hard, delete or rename the repo, change its visibility, commit .env or tokens.

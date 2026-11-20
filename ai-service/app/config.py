from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    # Env vars: OLLAMA_BASE_URL, OLLAMA_MODEL, OLLAMA_TIMEOUT_SECONDS.
    # Defaults fit a local Ollama, so no env is needed for development.
    model_config = SettingsConfigDict(env_prefix="OLLAMA_")

    base_url: str = "http://localhost:11434"
    model: str = "qwen3.5:4b"
    # A small local model can be slow on the first call, while it loads into memory.
    timeout_seconds: float = 60.0

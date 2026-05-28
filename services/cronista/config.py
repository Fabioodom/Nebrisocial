"""
config.py — Configuración centralizada del Agente Cronista.

Carga variables de entorno desde el archivo .env y expone el objeto
`settings` singleton con todos los parámetros del microservicio.
"""
from __future__ import annotations

import os
from dataclasses import dataclass, field
from dotenv import load_dotenv

load_dotenv()


@dataclass(frozen=True)
class Settings:
    """Configuración inmutable del Agente Cronista."""

    # ── Infraestructura ──────────────────────────────────────────────────────
    database_url: str = field(
        default_factory=lambda: os.getenv(
            "DATABASE_URL",
            "postgresql://nodal_user:nodal_password_dev@localhost:5432/nodal_db",
        )
    )

    # ── Proveedor LLM ─────────────────────────────────────────────────────────
    # "openai" → langchain-openai / "google" → langchain-google-genai
    llm_provider: str = field(
        default_factory=lambda: os.getenv("LLM_PROVIDER", "openai").lower()
    )
    llm_api_key: str = field(
        default_factory=lambda: os.getenv("LLM_API_KEY", "")
    )
    llm_model: str = field(
        default_factory=lambda: os.getenv("LLM_MODEL", "gpt-4o-mini")
    )

    # ── Parámetros de síntesis ────────────────────────────────────────────────
    min_messages_threshold: int = field(
        default_factory=lambda: int(os.getenv("MIN_MESSAGES_THRESHOLD", "20"))
    )
    max_summary_words: int = field(
        default_factory=lambda: int(os.getenv("MAX_SUMMARY_WORDS", "800"))
    )
    chunk_size_tokens: int = field(
        default_factory=lambda: int(os.getenv("CHUNK_SIZE_TOKENS", "500"))
    )

    # ── Programación del Cron ─────────────────────────────────────────────────
    cron_hour: int = field(
        default_factory=lambda: int(os.getenv("CRON_HOUR", "2"))
    )
    cron_minute: int = field(
        default_factory=lambda: int(os.getenv("CRON_MINUTE", "0"))
    )

    # ── Servidor MCP (Tool de lectura de chats) ───────────────────────────────
    mcp_host: str = field(
        default_factory=lambda: os.getenv("MCP_HOST", "0.0.0.0")
    )
    mcp_port: int = field(
        default_factory=lambda: int(os.getenv("MCP_PORT", "8002"))
    )


# Singleton global
settings = Settings()

"""
config.py — Configuración centralizada del Agente Curador.

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
    """Configuración inmutable del Agente Curador."""

    # ── Infraestructura ──────────────────────────────────────────────────────
    nats_url: str = field(
        default_factory=lambda: os.getenv("NATS_URL", "nats://localhost:4222")
    )
    database_url: str = field(
        default_factory=lambda: os.getenv(
            "DATABASE_URL",
            "postgresql://nodal_user:nodal_password_dev@localhost:5432/nodal_db",
        )
    )

    # ── Servidor MCP ──────────────────────────────────────────────────────────
    mcp_host: str = field(
        default_factory=lambda: os.getenv("MCP_HOST", "0.0.0.0")
    )
    mcp_port: int = field(
        default_factory=lambda: int(os.getenv("MCP_PORT", "8001"))
    )

    # ── APIs Externas ─────────────────────────────────────────────────────────
    rawg_api_key: str = field(
        default_factory=lambda: os.getenv("RAWG_API_KEY", "")
    )
    tmdb_api_key: str = field(
        default_factory=lambda: os.getenv("TMDB_API_KEY", "")
    )

    # ── Parámetros de comportamiento ──────────────────────────────────────────
    external_api_timeout_seconds: int = field(
        default_factory=lambda: int(os.getenv("EXTERNAL_API_TIMEOUT_SECONDS", "10"))
    )
    cache_ttl_seconds: int = field(
        default_factory=lambda: int(os.getenv("CACHE_TTL_SECONDS", "86400"))
    )

    # ── Mapa Categoría → Tools ────────────────────────────────────────────────
    # PRD Sección 6.4: qué herramientas invocar según la categoría del nodo.
    category_to_tools: dict[str, list[str]] = field(
        default_factory=lambda: {
            "manga": ["manga_metadata"],
            "anime": ["manga_metadata"],
            "videojuegos": ["game_metadata"],
            "cine": ["movie_metadata"],
            "musica": ["music_metadata"],
            "libros": ["book_metadata"],
            "pokemon": ["pokemon_metadata"],
            "tecnologia": ["tech_metadata"],
        }
    )


# Singleton global
settings = Settings()

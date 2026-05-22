"""
config.py — Configuración del Agente Guardián
================================================
Lee las variables de entorno con python-dotenv y expone un objeto Settings
con tipos validados. Todas las configuraciones son inmutables (frozen dataclass).
"""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from pathlib import Path

from dotenv import load_dotenv

# Carga el .env del directorio del proyecto (services/guardian/.env)
load_dotenv(dotenv_path=Path(__file__).parent / ".env")


@dataclass(frozen=True)
class Settings:
    """
    Objeto de configuración inmutable para el Agente Guardián.

    Attributes:
        nats_url: URL de conexión al servidor NATS.
        database_url: DSN de PostgreSQL en formato psycopg2.
        similarity_threshold_block: Similitud >= este valor → decisión BLOCK.
        similarity_threshold_suggest: Similitud >= este valor → decisión SUGGEST.
        embedding_model: Nombre del modelo sentence-transformers a cargar.
        max_similar_candidates: Número máximo de candidatos en respuesta SUGGEST.
    """

    nats_url: str = field(default_factory=lambda: _require("NATS_URL"))
    database_url: str = field(default_factory=lambda: _require("DATABASE_URL"))
    similarity_threshold_block: float = field(
        default_factory=lambda: float(os.getenv("SIMILARITY_THRESHOLD_BLOCK", "0.95"))
    )
    similarity_threshold_suggest: float = field(
        default_factory=lambda: float(os.getenv("SIMILARITY_THRESHOLD_SUGGEST", "0.85"))
    )
    embedding_model: str = field(
        default_factory=lambda: os.getenv("EMBEDDING_MODEL", "all-MiniLM-L6-v2")
    )
    max_similar_candidates: int = field(
        default_factory=lambda: int(os.getenv("MAX_SIMILAR_CANDIDATES", "5"))
    )


def _require(key: str) -> str:
    """
    Lee una variable de entorno obligatoria.

    Args:
        key: Nombre de la variable de entorno.

    Returns:
        Valor de la variable de entorno.

    Raises:
        ValueError: Si la variable no está definida o está vacía.
    """
    value = os.getenv(key, "").strip()
    if not value:
        raise ValueError(
            f"La variable de entorno obligatoria '{key}' no está definida. "
            f"Revisa tu archivo .env o las variables de entorno del sistema."
        )
    return value


# Instancia singleton de configuración, cargada al importar el módulo.
settings = Settings()

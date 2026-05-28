"""
config.py — Configuración centralizada del Agente Investigador de Frontends.

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
    """Configuración inmutable del Agente Investigador.

    Todos los campos se leen de variables de entorno con valores por defecto
    seguros. El dataclass es frozen (inmutable) para garantizar que la
    configuración no cambie en tiempo de ejecución.
    """

    # ── Proveedor LLM ─────────────────────────────────────────────────────────
    # "google" → langchain-google-genai (Gemini)
    # "openai" → langchain-openai (GPT)
    llm_provider: str = field(
        default_factory=lambda: os.getenv("LLM_PROVIDER", "google").lower()
    )
    llm_api_key: str = field(
        default_factory=lambda: os.getenv("LLM_API_KEY", "")
    )
    llm_model: str = field(
        default_factory=lambda: os.getenv("LLM_MODEL", "gemini-1.5-flash")
    )

    # ── Servidor FastMCP (SSE) ────────────────────────────────────────────────
    mcp_host: str = field(
        default_factory=lambda: os.getenv("MCP_HOST", "0.0.0.0")
    )
    mcp_port: int = field(
        default_factory=lambda: int(os.getenv("MCP_PORT", "8003"))
    )

    # ── Parámetros de Generación ──────────────────────────────────────────────
    max_retries: int = field(
        default_factory=lambda: int(os.getenv("MAX_RETRIES", "2"))
    )
    generation_timeout_seconds: int = field(
        default_factory=lambda: int(os.getenv("GENERATION_TIMEOUT_SECONDS", "30"))
    )

    # ── Design System ─────────────────────────────────────────────────────────
    design_system_ref_path: str = field(
        default_factory=lambda: os.getenv(
            "DESIGN_SYSTEM_REF_PATH", "./design_system_ref.md"
        )
    )

    def validate(self) -> None:
        """Valida que los campos críticos estén correctamente configurados.

        Raises:
            ValueError: Si LLM_API_KEY está vacía o el proveedor es inválido.
        """
        if not self.llm_api_key:
            raise ValueError(
                "❌ LLM_API_KEY no está configurada. "
                "Añade la clave de API del proveedor LLM al archivo .env"
            )
        if self.llm_provider not in {"google", "openai"}:
            raise ValueError(
                f"❌ LLM_PROVIDER='{self.llm_provider}' no es válido. "
                "Usa 'google' o 'openai'."
            )
        if self.generation_timeout_seconds < 10:
            raise ValueError(
                "❌ GENERATION_TIMEOUT_SECONDS debe ser >= 10 segundos. "
                "El ciclo generate+review necesita tiempo suficiente."
            )


# Singleton global — se instancia al importar el módulo
settings = Settings()

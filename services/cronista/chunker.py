"""
chunker.py — División de mensajes en chunks de tokens para el Agente Cronista.

Formatea los mensajes anonimizando a los usuarios (PRD R3) y los agrupa
en bloques de tamaño máximo `chunk_size_tokens` usando tiktoken para un
conteo de tokens preciso.
"""
from __future__ import annotations

import logging
from datetime import datetime
from typing import Any

import tiktoken

log = logging.getLogger(__name__)

# Encoding estándar compatible con GPT-4 / GPT-4o-mini y embeddings de OpenAI
_ENCODING_NAME = "cl100k_base"


def chunk_messages(
    messages: list[dict[str, Any]],
    chunk_size_tokens: int = 500,
) -> list[str]:
    """
    Agrupa mensajes en chunks de máximo `chunk_size_tokens` tokens.

    Formato de cada mensaje: ``"[HH:MM] Un participante dijo: {content}"``
    (sin exponer usernames reales, PRD R3).

    Args:
        messages: Lista de dicts con claves ``content`` y ``created_at``.
                  ``created_at`` puede ser un datetime o un string ISO-8601.
        chunk_size_tokens: Máximo de tokens por chunk (por defecto 500).

    Returns:
        Lista de strings; cada string es un chunk listo para enviar al LLM.
        Si no hay mensajes, devuelve una lista vacía.
    """
    if not messages:
        return []

    try:
        enc = tiktoken.get_encoding(_ENCODING_NAME)
    except Exception as exc:  # noqa: BLE001
        log.warning(
            "No se pudo cargar tiktoken (%s). Usando estimación por palabras.", exc
        )
        enc = None  # type: ignore[assignment]

    def count_tokens(text: str) -> int:
        """Cuenta tokens del texto; usa estimación si tiktoken no está disponible."""
        if enc is not None:
            return len(enc.encode(text))
        # Estimación conservadora: ~1.3 tokens por palabra
        return int(len(text.split()) * 1.3)

    def format_message(msg: dict[str, Any]) -> str:
        """Formatea un mensaje anonimizando al usuario."""
        raw_ts = msg.get("created_at")
        if isinstance(raw_ts, datetime):
            time_str = raw_ts.strftime("%H:%M")
        elif isinstance(raw_ts, str):
            # Intentar parsear ISO-8601
            try:
                dt = datetime.fromisoformat(raw_ts.replace("Z", "+00:00"))
                time_str = dt.strftime("%H:%M")
            except ValueError:
                time_str = "??:??"
        else:
            time_str = "??:??"

        content = (msg.get("content") or "").strip()
        return f"[{time_str}] Un participante dijo: {content}"

    chunks: list[str] = []
    current_lines: list[str] = []
    current_tokens: int = 0

    for msg in messages:
        line = format_message(msg)
        line_tokens = count_tokens(line)

        # Si un único mensaje supera el chunk_size, lo incluimos solo
        if line_tokens >= chunk_size_tokens:
            if current_lines:
                chunks.append("\n".join(current_lines))
                current_lines = []
                current_tokens = 0
            chunks.append(line)
            continue

        # Si añadir esta línea excede el límite, cerramos el chunk actual
        if current_tokens + line_tokens > chunk_size_tokens and current_lines:
            chunks.append("\n".join(current_lines))
            current_lines = []
            current_tokens = 0

        current_lines.append(line)
        current_tokens += line_tokens

    # Añadir el último chunk si tiene contenido
    if current_lines:
        chunks.append("\n".join(current_lines))

    log.info(
        "chunk_messages: %d mensajes → %d chunks (max %d tokens/chunk).",
        len(messages),
        len(chunks),
        chunk_size_tokens,
    )
    return chunks

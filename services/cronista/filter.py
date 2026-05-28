"""
filter.py — Filtrado de ruido en mensajes de chat del Agente Cronista.

Elimina mensajes de baja calidad antes de enviarlos al LLM para síntesis,
reduciendo el coste de tokens y mejorando la calidad del resumen.
"""
from __future__ import annotations

import re
import logging
from typing import Any

log = logging.getLogger(__name__)

# ── Expresiones regulares ──────────────────────────────────────────────────────

# Detecta cadenas que contienen SOLO emojis y/o caracteres no alfanuméricos
_RE_ONLY_NON_ALNUM = re.compile(r"^[^\w]+$", re.UNICODE)

# Detecta cadenas que son SOLO una URL (sin texto adicional)
_RE_ONLY_URL = re.compile(
    r"^\s*(https?://[^\s]+|www\.[^\s]+)\s*$",
    re.IGNORECASE,
)


def filter_noise(messages: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """
    Filtra mensajes de baja calidad antes de enviarlos al LLM.

    Criterios de eliminación:
      1. Menos de 5 palabras.
      2. Solo emojis o caracteres no alfanuméricos (sin letras/números).
      3. Mensajes duplicados consecutivos del mismo usuario.
      4. Mensajes que contienen solo una URL sin texto adicional.

    El orden cronológico de los mensajes restantes se preserva.

    Args:
        messages: Lista de dicts con al menos las claves
                  ``content`` (str) y ``username`` (str).

    Returns:
        Lista filtrada manteniendo el orden original.
    """
    if not messages:
        return []

    filtered: list[dict[str, Any]] = []
    prev_content: str | None = None
    prev_username: str | None = None
    discarded = 0

    for msg in messages:
        content: str = (msg.get("content") or "").strip()
        username: str = str(msg.get("username") or "")

        # ── Filtro 1: menos de 5 palabras ────────────────────────────────────
        word_count = len(content.split())
        if word_count < 5:
            discarded += 1
            continue

        # ── Filtro 2: solo emojis / caracteres no alfanuméricos ──────────────
        if _RE_ONLY_NON_ALNUM.match(content):
            discarded += 1
            continue

        # ── Filtro 3: solo URL sin texto ─────────────────────────────────────
        if _RE_ONLY_URL.match(content):
            discarded += 1
            continue

        # ── Filtro 4: duplicado consecutivo del mismo usuario ─────────────────
        if content == prev_content and username == prev_username:
            discarded += 1
            continue

        filtered.append(msg)
        prev_content = content
        prev_username = username

    log.info(
        "filter_noise: %d mensajes originales → %d conservados (%d descartados).",
        len(messages),
        len(filtered),
        discarded,
    )
    return filtered

"""
decision.py — Lógica de Decisión del Agente Guardián
======================================================
Función pura ``evaluate_similarity`` que aplica los umbrales del PRD
(Sección 6.2) y devuelve un dict de decisión estructurado.

Esta función NO tiene efectos secundarios:
  - No accede a la base de datos.
  - No llama a servicios externos.
  - No modifica estado global.

Esto la hace completamente testeable con pytest sin mocks.
"""

from __future__ import annotations

from typing import Any

from config import Settings


def evaluate_similarity(
    similar_nodes: list[dict[str, Any]],
    config: Settings,
    timeout: bool = False,
) -> dict[str, Any]:
    """
    Evalúa la lista de nodos similares y devuelve la decisión del guardián.

    Los umbrales aplicados (PRD Sección 6.2, Reglas R1-R3):
      - R1 (BLOCK estricto): similarity > SIMILARITY_THRESHOLD_BLOCK → BLOCK,
        sin excepciones salvo timeout SIN hits por encima del umbral.
      - R2 (SUGGEST): SIMILARITY_THRESHOLD_SUGGEST < similarity <= SIMILARITY_THRESHOLD_BLOCK
        → SUGGEST con lista de candidatos.
      - R3 (APPROVE con revisión): timeout=True y el mejor hit (si lo hay) NO
        supera SIMILARITY_THRESHOLD_BLOCK → APPROVE con needs_review=True.
      - DEFAULT: sin candidatos o mejor similitud < SIMILARITY_THRESHOLD_SUGGEST
        → APPROVE limpio.

    Args:
        similar_nodes: Lista de dicts devuelta por VectorRepository.search_similar_nodes().
                       Cada dict contiene: node_id, title, slug, similarity.
                       Se asume ordenada de mayor a menor similitud.
        config: Objeto Settings con los umbrales de configuración.
        timeout: True si la consulta pgvector superó el timeout de 2 s (PRD R3).

    Returns:
        Dict con al menos la clave ``"decision"`` (str: "approve" | "block" | "suggest").
        Estructura completa:

        BLOCK:
          {
            "decision": "block",
            "similar_node": {"node_id": ..., "title": ..., "slug": ..., "similarity": ...},
            "reason": "Nodo semánticamente idéntico ya existe."
          }

        SUGGEST:
          {
            "decision": "suggest",
            "candidates": [...],  # hasta MAX_SIMILAR_CANDIDATES entradas
            "reason": "Existen Nodos similares. Considera unirte a uno de ellos."
          }

        APPROVE (limpio):
          {"decision": "approve"}

        APPROVE (con revisión — degradación elegante o timeout sin hit alto):
          {"decision": "approve", "needs_review": True}

    Raises:
        No lanza excepciones — errores deben capturarse en el llamador.
    """
    # Extraer el candidato con mayor similitud (el primero de la lista ordenada)
    best_candidate: dict[str, Any] | None = similar_nodes[0] if similar_nodes else None
    best_similarity: float = float(best_candidate["similarity"]) if best_candidate else 0.0

    # ── R1: BLOCK estricto ──────────────────────────────────────────────────
    # Si existe algún candidato con similitud > umbral de bloqueo, SIEMPRE se
    # bloquea — incluso si hubo timeout (PRD R1 es absoluto salvo no hayamos
    # visto el hit todavía, pero si está en la respuesta parcial, se bloquea).
    if best_similarity > config.similarity_threshold_block:
        return {
            "decision": "block",
            "similar_node": {
                "node_id": best_candidate["node_id"],       # type: ignore[index]
                "title":   best_candidate["title"],          # type: ignore[index]
                "slug":    best_candidate["slug"],           # type: ignore[index]
                "similarity": round(best_similarity, 6),
            },
            "reason": "Nodo semánticamente idéntico ya existe.",
        }

    # ── R3: Timeout sin hit de bloqueo ──────────────────────────────────────
    # La consulta tardó demasiado y no encontramos ningún hit por encima del
    # umbral de bloqueo → aprobamos con flag de revisión humana (PRD R3).
    if timeout:
        return {
            "decision": "approve",
            "needs_review": True,
        }

    # ── R2: SUGGEST ─────────────────────────────────────────────────────────
    # Hay candidatos moderadamente similares (entre suggest y block thresholds).
    if best_similarity > config.similarity_threshold_suggest:
        candidates = [
            {
                "node_id":    node["node_id"],
                "title":      node["title"],
                "slug":       node["slug"],
                "similarity": round(float(node["similarity"]), 6),
            }
            for node in similar_nodes[: config.max_similar_candidates]
        ]
        return {
            "decision": "suggest",
            "candidates": candidates,
            "reason": "Existen Nodos similares. Considera unirte a uno de ellos.",
        }

    # ── DEFAULT: APPROVE limpio ──────────────────────────────────────────────
    return {"decision": "approve"}

"""
curator_agent.py — Lógica de orquestación del Agente Curador.

La clase CuratorAgent recibe el contexto configurado (Settings, NodeRepository
y el cliente MCP) y expone el método `curate()` que implementa el flujo
completo del PRD Sección 6.4:

  1. Determina qué tools invocar según la categoría del nodo.
  2. Invoca cada tool de forma asíncrona, respetando el timeout.
  3. Fusiona los resultados en un único dict de metadatos enriquecidos.
  4. Protege las claves editadas manualmente (PRD R2).
  5. Actualiza nodes.metadata con el operador JSONB ||.
  6. Registra la acción en agent_audit_log.
"""
from __future__ import annotations

import asyncio
import logging
from typing import Any

from config import Settings
from node_repo import NodeRepository

log = logging.getLogger(__name__)

# Mapa local de nombres de tool → función coroutine importada del mcp_server.
# Se resuelve en tiempo de ejecución para evitar importaciones circulares.
_TOOL_REGISTRY: dict[str, Any] = {}


def _get_tool_fn(tool_name: str) -> Any:
    """Resuelve el nombre de la tool al callable real del mcp_server."""
    global _TOOL_REGISTRY
    if not _TOOL_REGISTRY:
        from mcp_server import (
            manga_metadata,
            game_metadata,
            movie_metadata,
            music_metadata,
            book_metadata,
            pokemon_metadata,
            tech_metadata,
        )

        _TOOL_REGISTRY = {
            "manga_metadata": manga_metadata,
            "game_metadata": game_metadata,
            "movie_metadata": movie_metadata,
            "music_metadata": music_metadata,
            "book_metadata": book_metadata,
            "pokemon_metadata": pokemon_metadata,
            "tech_metadata": tech_metadata,
        }
    fn = _TOOL_REGISTRY.get(tool_name)
    if fn is None:
        raise KeyError(f"Tool '{tool_name}' no está registrada en _TOOL_REGISTRY.")
    return fn


class CuratorAgent:
    """
    Orquesta el enriquecimiento de metadatos de un nodo recién creado.

    Attributes:
        settings: Objeto de configuración inmutable.
        repo: Repositorio de acceso a la base de datos.
    """

    def __init__(self, settings: Settings, repo: NodeRepository) -> None:
        """
        Inicializa el agente con su configuración y repositorio.

        Args:
            settings: Instancia de Settings con los parámetros del servicio.
            repo: Instancia de NodeRepository para leer/escribir en PostgreSQL.
        """
        self.settings = settings
        self.repo = repo

    async def curate(
        self, node_id: str, title: str, category: str
    ) -> None:
        """
        Flujo principal de curación: enriquece los metadatos de un nodo.

        Args:
            node_id: UUID del nodo recién creado.
            title: Título del nodo (usado como argumento para las tools).
            category: Categoría del nodo (normalizada a minúsculas internamente).

        Raises:
            No lanza excepciones hacia afuera — todos los errores se capturan y
            registran para cumplir con PRD R4 (degradación elegante).
        """
        normalized_category = category.lower().strip()
        log.info(
            "CuratorAgent.curate iniciado — node_id=%s, category='%s'",
            node_id,
            normalized_category,
        )

        # ── Paso 1: Determinar tools relevantes ───────────────────────────────
        tool_names: list[str] = self.settings.category_to_tools.get(
            normalized_category, []
        )

        if not tool_names:
            log.warning(
                "No hay tools mapeadas para la categoría '%s'. "
                "El nodo %s no recibirá metadatos automáticos.",
                normalized_category,
                node_id,
            )
            self.repo.log_audit(
                agent_type="curator",
                action="no_tools_available",
                input_data={
                    "node_id": node_id,
                    "category": normalized_category,
                    "tools_used": [],
                },
                output_data={"enriched_fields": []},
                node_id=node_id,
            )
            return

        log.info(
            "Tools seleccionadas para '%s': %s", normalized_category, tool_names
        )

        # ── Paso 2: Obtener metadata actual del nodo (proteger claves manuales)
        try:
            node_data = self.repo.get_node(node_id)
            existing_metadata: dict[str, Any] = node_data.get("metadata") or {}
        except Exception as exc:  # noqa: BLE001
            log.error(
                "No se pudo leer el nodo %s: %s. Abortando curación.", node_id, exc
            )
            return

        # ── Paso 3: Invocar cada tool respetando timeout y reintentos ─────────
        enriched_metadata: dict[str, Any] = {}
        tools_used: list[str] = []

        for tool_name in tool_names:
            try:
                fn = _get_tool_fn(tool_name)
            except KeyError as exc:
                log.error("Tool desconocida '%s': %s. Saltando.", tool_name, exc)
                continue

            # PRD R3: timeout estricto por tool
            try:
                result: dict[str, Any] = await asyncio.wait_for(
                    fn(title),  # Todas las tools reciben `title` como argumento
                    timeout=float(self.settings.external_api_timeout_seconds),
                )
                enriched_metadata.update(result)
                tools_used.append(tool_name)
                log.info(
                    "Tool '%s' ejecutada con éxito para node_id=%s", tool_name, node_id
                )
            except asyncio.TimeoutError:
                # PRD R3 + R4: timeout → loguear y continuar
                log.warning(
                    "Tool '%s' excedió el timeout de %ds para node_id=%s. Saltando.",
                    tool_name,
                    self.settings.external_api_timeout_seconds,
                    node_id,
                )
            except Exception as exc:  # noqa: BLE001
                # PRD R4: cualquier fallo de API → loguear y continuar
                log.warning(
                    "Tool '%s' falló para node_id=%s: %s. Saltando.",
                    tool_name,
                    node_id,
                    exc,
                )

        if not enriched_metadata:
            log.warning(
                "Ninguna tool devolvió datos para node_id=%s. "
                "El nodo se mantiene sin metadatos automáticos.",
                node_id,
            )
            self.repo.log_audit(
                agent_type="curator",
                action="no_metadata_obtained",
                input_data={
                    "node_id": node_id,
                    "category": normalized_category,
                    "tools_used": tools_used,
                },
                output_data={"enriched_fields": []},
                node_id=node_id,
            )
            return

        # ── Paso 4: Proteger claves editadas manualmente (PRD R2) ─────────────
        # El operador JSONB || en PostgreSQL ya garantiza esto a nivel DB, pero
        # aquí aplicamos una capa extra: eliminamos de enriched_metadata cualquier
        # clave que YA exista en existing_metadata (asumida como edición manual).
        manually_set_keys = set(existing_metadata.keys())
        conflicting_keys = manually_set_keys.intersection(enriched_metadata.keys())
        if conflicting_keys:
            log.info(
                "Preservando claves con valores manuales para node_id=%s: %s",
                node_id,
                conflicting_keys,
            )
            for key in conflicting_keys:
                del enriched_metadata[key]

        if not enriched_metadata:
            log.info(
                "Todos los campos enriquecidos ya existían manualmente en node_id=%s. "
                "No se escribirá nada.",
                node_id,
            )
            return

        # ── Paso 5: Persistir los metadatos enriquecidos ──────────────────────
        try:
            self.repo.update_node_metadata(node_id, enriched_metadata)
        except Exception as exc:  # noqa: BLE001
            log.error(
                "No se pudo actualizar metadata de node_id=%s: %s", node_id, exc
            )
            # PRD R4: fallos de escritura no bloquean el nodo
            self.repo.log_audit(
                agent_type="curator",
                action="metadata_write_failed",
                input_data={
                    "node_id": node_id,
                    "category": normalized_category,
                    "tools_used": tools_used,
                },
                output_data={"error": str(exc)},
                node_id=node_id,
            )
            return

        # ── Paso 6: Registrar en audit log ────────────────────────────────────
        self.repo.log_audit(
            agent_type="curator",
            action="metadata_enriched",
            input_data={
                "node_id": node_id,
                "category": normalized_category,
                "tools_used": tools_used,
            },
            output_data={"enriched_fields": list(enriched_metadata.keys())},
            node_id=node_id,
        )

        log.info(
            "CuratorAgent.curate completado — node_id=%s, campos enriquecidos: %s",
            node_id,
            list(enriched_metadata.keys()),
        )

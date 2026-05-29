"""
mcp_server.py — Servidor FastMCP del Agente Cronista.

Expone una Tool MCP para que otros agentes o moderadores puedan leer
los últimos chats de un Nodo directamente desde PostgreSQL.

Tool disponible:
  - get_node_recent_chats(node_id, hours): devuelve los mensajes de un nodo.
  - list_active_nodes(min_messages): lista nodos con actividad reciente.
"""
from __future__ import annotations

import logging
from typing import Any

from fastmcp import FastMCP

from config import settings
from message_repo import MessageRepository

log = logging.getLogger(__name__)

# ── Instancia FastMCP ──────────────────────────────────────────────────────────
mcp = FastMCP(
    name="cronista-mcp",
    instructions=(
        "Servidor MCP del Agente Cronista de Nodal. "
        "Permite leer conversaciones de chat de nodos para síntesis y moderación."
    ),
)

# Repositorio compartido (inicializado en tiempo de arranque)
_repo: MessageRepository | None = None


def init_repo(repo: MessageRepository) -> None:
    """Inyecta el repositorio de mensajes en el servidor MCP."""
    global _repo  # noqa: PLW0603
    _repo = repo


def _get_repo() -> MessageRepository:
    """Devuelve el repositorio o lanza error si no está inicializado."""
    if _repo is None:
        raise RuntimeError(
            "MessageRepository no inicializado. Llama a init_repo() primero."
        )
    return _repo


# ── Tools MCP ─────────────────────────────────────────────────────────────────

@mcp.tool()
def get_node_recent_chats(
    node_id: str,
    hours: int = 24,
) -> dict[str, Any]:
    """
    Lee los mensajes de chat de un Nodo en las últimas N horas.

    Los usernames son anonimizados en el chunker antes de la síntesis.
    Esta tool devuelve los mensajes en crudo para que el agente externo
    pueda procesarlos como considere oportuno.

    Args:
        node_id: UUID del nodo del que se quieren leer los chats.
        hours: Ventana temporal en horas hacia atrás (por defecto 24).

    Returns:
        Dict con:
          - node_id (str)
          - hours (int)
          - message_count (int)
          - messages (list of dict con content, username, created_at)
    """
    log.info("MCP tool get_node_recent_chats: node_id=%s, hours=%d", node_id, hours)
    try:
        repo = _get_repo()
        messages = repo.get_messages_for_node(node_id=node_id, since_hours=hours)
        return {
            "node_id": node_id,
            "hours": hours,
            "message_count": len(messages),
            "messages": [
                {
                    "content": m.get("content", ""),
                    "username": m.get("username", "anonymous"),
                    "created_at": (
                        m["created_at"].isoformat()
                        if hasattr(m.get("created_at"), "isoformat")
                        else str(m.get("created_at", ""))
                    ),
                }
                for m in messages
            ],
        }
    except Exception as exc:  # noqa: BLE001
        log.error("get_node_recent_chats error: %s", exc)
        return {
            "node_id": node_id,
            "hours": hours,
            "message_count": 0,
            "messages": [],
            "error": str(exc),
        }


@mcp.tool()
def list_active_nodes(min_messages: int = 20) -> dict[str, Any]:
    """
    Lista los nodos que tuvieron más de `min_messages` en las últimas 24h
    y aún no tienen un resumen IA generado en ese período.

    Args:
        min_messages: Umbral mínimo de mensajes para incluir un nodo.

    Returns:
        Dict con:
          - node_count (int)
          - nodes (list of dict con id, title)
    """
    log.info("MCP tool list_active_nodes: min_messages=%d", min_messages)
    try:
        repo = _get_repo()
        nodes = repo.get_active_nodes_with_messages(min_messages=min_messages)
        return {
            "node_count": len(nodes),
            "nodes": [{"id": n["id"], "title": n["title"]} for n in nodes],
        }
    except Exception as exc:  # noqa: BLE001
        log.error("list_active_nodes error: %s", exc)
        return {
            "node_count": 0,
            "nodes": [],
            "error": str(exc),
        }

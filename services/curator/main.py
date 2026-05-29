"""
main.py — Punto de entrada del Agente Curador.

Arranca dos tareas concurrentes con asyncio.gather():
  1. El servidor FastMCP en modo SSE (HTTP) en MCP_HOST:MCP_PORT.
  2. El suscriptor NATS que escucha `node.created` e invoca CuratorAgent.

Degradación elegante (PRD R4):
  - Si NATS no está disponible al iniciar, el proceso informa y reintenta
    la conexión. El servidor MCP sigue funcionando independientemente.
  - Si una llamada a curate() falla, se captura la excepción, se registra
    y el mensaje NATS se descarta sin bloquear al broker.
"""
from __future__ import annotations

import asyncio
import json
import logging
import signal
import sys
from typing import Any

import nats
import nats.errors
from nats.aio.client import Client as NatsClient
from nats.aio.msg import Msg

from config import settings
from curator_agent import CuratorAgent
from mcp_server import mcp
from node_repo import NodeRepository

# ── Logging ───────────────────────────────────────────────────────────────────
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
    stream=sys.stdout,
)
log = logging.getLogger("curator.main")

# ── Subjects NATS ─────────────────────────────────────────────────────────────
SUBJECT_NODE_CREATED = "node.created"


# ─────────────────────────────────────────────────────────────────────────────
# Suscriptor NATS
# ─────────────────────────────────────────────────────────────────────────────

async def run_nats_subscriber(agent: CuratorAgent) -> None:
    """
    Conecta a NATS y suscribe al subject `node.created`.

    El handler deserializa el payload JSON y delega a CuratorAgent.curate().
    Si la conexión se pierde, NATS-py intenta reconectar automáticamente.

    Args:
        agent: Instancia de CuratorAgent ya configurada.
    """
    nc: NatsClient | None = None

    async def curator_handler(msg: Msg) -> None:
        """Procesa cada mensaje recibido en el subject node.created."""
        raw_payload = msg.data.decode("utf-8")
        log.info("NATS [%s] mensaje recibido: %s", SUBJECT_NODE_CREATED, raw_payload)

        try:
            payload: dict[str, Any] = json.loads(raw_payload)
        except json.JSONDecodeError as exc:
            log.error("Payload JSON inválido: %s — %s", raw_payload, exc)
            return

        node_id: str | None = payload.get("node_id")
        title: str | None = payload.get("title")
        category: str | None = payload.get("category")

        if not node_id or not title or not category:
            log.error(
                "Payload incompleto (faltan campos node_id/title/category): %s",
                payload,
            )
            return

        # PRD R4: si curate() falla, el nodo se crea sin metadatos.
        try:
            await agent.curate(node_id=node_id, title=title, category=category)
        except Exception as exc:  # noqa: BLE001
            log.error(
                "curate() falló para node_id=%s: %s — el nodo se mantiene sin metadatos.",
                node_id,
                exc,
            )

    # Intentar conectar con reintentos en bucle
    retry_delay = 5  # segundos entre intentos
    while True:
        try:
            # Intentar conectar con reintentos en bucle
            log.info("Conectando a NATS en %s ...", settings.nats_url)
            
            # Funciones asíncronas para los eventos de NATS
            async def error_cb(exc):
                log.error("NATS error: %s", exc)
            async def closed_cb():
                log.warning("NATS: conexión cerrada.")
            async def reconnected_cb():
                log.info("NATS: reconectado.")
            async def disconnected_cb():
                log.warning("NATS: desconectado.")

            nc = await nats.connect(
                servers=settings.nats_url,
                error_cb=error_cb,
                closed_cb=closed_cb,
                reconnected_cb=reconnected_cb,
                disconnected_cb=disconnected_cb,
                max_reconnect_attempts=-1,   # reconexión indefinida
                reconnect_time_wait=2,
            )
            log.info("NATS conectado. Suscribiendo a '%s'...", SUBJECT_NODE_CREATED)
            await nc.subscribe(SUBJECT_NODE_CREATED, cb=curator_handler)
            log.info(
                "Agente Curador escuchando en NATS subject '%s'.",
                SUBJECT_NODE_CREATED,
            )
            # Mantener el loop vivo hasta que se cierre la conexión
            while nc.is_connected:
                await asyncio.sleep(1)

        except nats.errors.NoServersError as exc:
            log.warning(
                "No se pudo conectar a NATS (%s): %s. Reintentando en %ds...",
                settings.nats_url,
                exc,
                retry_delay,
            )
            await asyncio.sleep(retry_delay)
        except Exception as exc:  # noqa: BLE001
            log.error("Error inesperado en el suscriptor NATS: %s", exc)
            await asyncio.sleep(retry_delay)
        finally:
            if nc and not nc.is_closed:
                await nc.close()


# ─────────────────────────────────────────────────────────────────────────────
# Servidor FastMCP (SSE / HTTP)
# ─────────────────────────────────────────────────────────────────────────────

async def run_mcp_server() -> None:
    """
    Arranca el servidor FastMCP en modo SSE sobre HTTP.

    El servidor queda disponible en http://MCP_HOST:MCP_PORT/sse para que
    otros agentes o clientes MCP puedan invocar las tools.
    """
    log.info(
        "Iniciando servidor FastMCP en http://%s:%d",
        settings.mcp_host,
        settings.mcp_port,
    )
    await mcp.run_async(
        transport="sse",
        host=settings.mcp_host,
        port=settings.mcp_port,
    )


# ─────────────────────────────────────────────────────────────────────────────
# Punto de entrada principal
# ─────────────────────────────────────────────────────────────────────────────

async def _async_main() -> None:
    """Punto de entrada asíncrono: arranca el servidor MCP y el suscriptor NATS."""
    log.info("=" * 60)
    log.info("  Agente Curador de Nodal — Iniciando")
    log.info("  NATS URL  : %s", settings.nats_url)
    log.info("  MCP       : http://%s:%d", settings.mcp_host, settings.mcp_port)
    log.info("=" * 60)

    # Inicializar repositorio de base de datos
    try:
        repo = NodeRepository(database_url=settings.database_url)
    except Exception as exc:  # noqa: BLE001
        log.critical("No se pudo conectar a PostgreSQL: %s. Abortando.", exc)
        sys.exit(1)

    # Inicializar el agente curador
    agent = CuratorAgent(settings=settings, repo=repo)

    # Manejo de señales para cierre ordenado
    loop = asyncio.get_running_loop()

    def _shutdown() -> None:
        log.info("Señal de cierre recibida. Deteniendo el Agente Curador...")
        for task in asyncio.all_tasks(loop):
            task.cancel()

    for sig in (signal.SIGINT, signal.SIGTERM):
        try:
            loop.add_signal_handler(sig, _shutdown)
        except (NotImplementedError, OSError):
            # Windows no soporta add_signal_handler para todos los signals
            pass

    # Ejecutar servidor MCP y suscriptor NATS de forma concurrente
    try:
        await asyncio.gather(
            run_mcp_server(),
            run_nats_subscriber(agent),
        )
    except asyncio.CancelledError:
        log.info("Agente Curador detenido correctamente.")
    finally:
        repo.close()


if __name__ == "__main__":
    asyncio.run(_async_main())

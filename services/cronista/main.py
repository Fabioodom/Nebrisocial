"""
main.py — Punto de entrada del Agente Cronista.

Arranca dos tareas concurrentes con asyncio:
  1. APScheduler (AsyncIOScheduler) que ejecuta run_daily_summary() a las
     CRON_HOUR:CRON_MINUTE UTC cada día.
  2. El servidor FastMCP en modo SSE (HTTP) en MCP_HOST:MCP_PORT para
     que otros agentes puedan leer chats vía la Tool MCP.

Modo CLI (disparo manual):
  python -m main --run-now
  Ejecuta run_daily_summary() una vez inmediatamente y termina.

Flujo de run_daily_summary() (PRD Sección 6.3):
  a. Obtener nodos activos con get_active_nodes_with_messages.
  b. Para cada nodo:
     i.   Recuperar mensajes con get_messages_for_node.
     ii.  Filtrar ruido con filter_noise.
     iii. Generar chunks con chunk_messages.
     iv.  Generar resumen con Summarizer.summarize.
     v.   Publicar el hilo con save_summary_thread.
     vi.  Registrar en agent_audit_log.
  c. Si un nodo falla, loguear y continuar (no detener el job).
"""
from __future__ import annotations

import argparse
import asyncio
import logging
import signal
import sys
from datetime import date

from apscheduler.schedulers.asyncio import AsyncIOScheduler

from chunker import chunk_messages
from config import settings
from filter import filter_noise
from message_repo import MessageRepository
from mcp_server import init_repo, mcp
from summarizer import Summarizer

# ── Logging ───────────────────────────────────────────────────────────────────
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
    stream=sys.stdout,
)
log = logging.getLogger("cronista.main")

# ── Constantes ────────────────────────────────────────────────────────────────
AGENT_TYPE = "cronista"


# ─────────────────────────────────────────────────────────────────────────────
# Lógica del cron job
# ─────────────────────────────────────────────────────────────────────────────

async def run_daily_summary(
    repo: MessageRepository,
    summarizer: Summarizer,
) -> None:
    """
    Orquesta la síntesis diaria para TODOS los nodos activos.

    Itera secuencialmente para no sobrecargar la API del LLM.
    Si un nodo falla, registra el error y continúa con el siguiente.

    Args:
        repo: Repositorio de mensajes (acceso a PostgreSQL).
        summarizer: Instancia de Summarizer (LangChain Map-Reduce).
    """
    today_str = date.today().isoformat()
    log.info("=" * 60)
    log.info("  Cronista: iniciando síntesis diaria — %s", today_str)
    log.info("=" * 60)

    # ── a. Obtener nodos activos ───────────────────────────────────────────────
    try:
        active_nodes = repo.get_active_nodes_with_messages(
            min_messages=settings.min_messages_threshold
        )
    except Exception as exc:  # noqa: BLE001
        log.error(
            "No se pudo obtener la lista de nodos activos: %s. Abortando ciclo.",
            exc,
        )
        return

    if not active_nodes:
        log.info(
            "No hay nodos activos con más de %d mensajes hoy. "
            "Nada que resumir.",
            settings.min_messages_threshold,
        )
        return

    log.info("Nodos a procesar: %d", len(active_nodes))

    success_count = 0
    failure_count = 0

    # ── b. Procesar cada nodo ─────────────────────────────────────────────────
    for node in active_nodes:
        node_id: str = str(node["id"])
        node_title: str = str(node["title"])

        log.info("--- Procesando nodo: '%s' (%s) ---", node_title, node_id)

        try:
            # i. Recuperar mensajes
            messages = repo.get_messages_for_node(node_id=node_id, since_hours=24)
            message_count = len(messages)
            log.info("  Mensajes recuperados: %d", message_count)

            # ii. Filtrar ruido
            clean_messages = filter_noise(messages)
            log.info("  Mensajes tras filtrado: %d", len(clean_messages))

            if not clean_messages:
                log.warning(
                    "  Nodo '%s': todos los mensajes fueron filtrados. Omitiendo.",
                    node_title,
                )
                repo.log_audit(
                    agent_type=AGENT_TYPE,
                    action="summary_skipped",
                    input_data={"node_id": node_id, "message_count": message_count},
                    output_data={"reason": "all_messages_filtered"},
                    node_id=node_id,
                )
                continue

            # iii. Generar chunks
            chunks = chunk_messages(
                clean_messages,
                chunk_size_tokens=settings.chunk_size_tokens,
            )
            log.info("  Chunks generados: %d", len(chunks))

            # iv. Generar resumen con LLM
            summary_body = summarizer.summarize(
                node_title=node_title,
                chunks=chunks,
            )
            word_count = len(summary_body.split())
            log.info("  Resumen generado: %d palabras.", word_count)

            # v. Publicar el hilo
            thread_title = f"[Resumen IA] {node_title} — {today_str}"
            thread_id = repo.save_summary_thread(
                node_id=node_id,
                title=thread_title,
                body=summary_body,
            )
            log.info("  Hilo publicado: thread_id=%s", thread_id)

            # vi. Registrar auditoría exitosa
            repo.log_audit(
                agent_type=AGENT_TYPE,
                action="summary_created",
                input_data={
                    "node_id": node_id,
                    "message_count": message_count,
                    "clean_message_count": len(clean_messages),
                    "chunk_count": len(chunks),
                },
                output_data={
                    "thread_id": thread_id,
                    "thread_title": thread_title,
                    "word_count": word_count,
                },
                node_id=node_id,
            )
            success_count += 1

        except Exception as exc:  # noqa: BLE001
            # c. Error por nodo: loguear y continuar (PRD R5)
            log.error(
                "  ERROR procesando nodo '%s' (%s): %s",
                node_title,
                node_id,
                exc,
                exc_info=True,
            )
            try:
                repo.log_audit(
                    agent_type=AGENT_TYPE,
                    action="summary_failed",
                    input_data={"node_id": node_id, "node_title": node_title},
                    output_data={"error": str(exc)},
                    node_id=node_id,
                )
            except Exception as audit_exc:  # noqa: BLE001
                log.warning("No se pudo registrar el fallo en auditoría: %s", audit_exc)
            failure_count += 1
            continue

    log.info(
        "Síntesis diaria completada — Éxitos: %d | Fallos: %d | Total: %d",
        success_count,
        failure_count,
        len(active_nodes),
    )


# ─────────────────────────────────────────────────────────────────────────────
# Servidor FastMCP
# ─────────────────────────────────────────────────────────────────────────────

async def run_mcp_server() -> None:
    """
    Arranca el servidor FastMCP en modo SSE sobre HTTP.

    Disponible en http://MCP_HOST:MCP_PORT/sse para clientes MCP externos.
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

async def _async_main(run_now: bool = False) -> None:
    """
    Punto de entrada asíncrono.

    Args:
        run_now: Si True, ejecuta run_daily_summary() inmediatamente y termina.
                 Si False, arranca el scheduler y el servidor MCP.
    """
    log.info("=" * 60)
    log.info("  Agente Cronista de Nodal — Iniciando")
    log.info("  DB   : %s", settings.database_url.split("@")[-1])
    log.info("  LLM  : %s / %s", settings.llm_provider, settings.llm_model)
    log.info(
        "  CRON : %02d:%02d UTC diariamente",
        settings.cron_hour,
        settings.cron_minute,
    )
    log.info("  MCP  : http://%s:%d", settings.mcp_host, settings.mcp_port)
    log.info("=" * 60)

    # Inicializar repositorio
    try:
        repo = MessageRepository(database_url=settings.database_url)
    except Exception as exc:  # noqa: BLE001
        log.critical("No se pudo conectar a PostgreSQL: %s. Abortando.", exc)
        sys.exit(1)

    # Inicializar Summarizer
    if not settings.llm_api_key:
        log.warning(
            "LLM_API_KEY no configurada. El Summarizer fallará en tiempo de ejecución."
        )
    summarizer = Summarizer(
        provider=settings.llm_provider,
        api_key=settings.llm_api_key,
        model=settings.llm_model,
        max_words=settings.max_summary_words,
    )

    # Inyectar repositorio en el servidor MCP
    init_repo(repo)

    # ── Modo: disparo manual ──────────────────────────────────────────────────
    if run_now:
        log.info("Modo --run-now: ejecutando síntesis diaria inmediatamente...")
        await run_daily_summary(repo=repo, summarizer=summarizer)
        repo.close()
        log.info("Cronista --run-now completado. Saliendo.")
        return

    # ── Modo: servicio continuo (scheduler + MCP) ─────────────────────────────
    scheduler = AsyncIOScheduler(timezone="UTC")
    scheduler.add_job(
        func=run_daily_summary,
        trigger="cron",
        hour=settings.cron_hour,
        minute=settings.cron_minute,
        id="daily_summary",
        name="Síntesis Diaria de Nodos",
        kwargs={"repo": repo, "summarizer": summarizer},
        misfire_grace_time=3600,  # 1 hora de gracia si el sistema estuvo caído
        coalesce=True,             # si se perdieron ejecuciones, ejecutar solo 1
    )
    scheduler.start()
    log.info(
        "APScheduler iniciado. Próxima ejecución: %s",
        scheduler.get_job("daily_summary").next_run_time,
    )

    # Manejo de señales para cierre ordenado
    loop = asyncio.get_running_loop()

    def _shutdown() -> None:
        log.info("Señal de cierre recibida. Deteniendo el Agente Cronista...")
        scheduler.shutdown(wait=False)
        for task in asyncio.all_tasks(loop):
            task.cancel()

    for sig in (signal.SIGINT, signal.SIGTERM):
        try:
            loop.add_signal_handler(sig, _shutdown)
        except (NotImplementedError, OSError):
            # Windows no soporta add_signal_handler para algunos signals
            pass

    try:
        await run_mcp_server()
    except asyncio.CancelledError:
        log.info("Agente Cronista detenido correctamente.")
    finally:
        scheduler.shutdown(wait=False)
        repo.close()


def main() -> None:
    """Punto de entrada sincrónico. Parsea argumentos CLI y lanza asyncio."""
    parser = argparse.ArgumentParser(
        description="Agente Cronista — Síntesis Diaria de Nodos"
    )
    parser.add_argument(
        "--run-now",
        action="store_true",
        help="Ejecutar la síntesis diaria inmediatamente y salir.",
    )
    args = parser.parse_args()

    asyncio.run(_async_main(run_now=args.run_now))


if __name__ == "__main__":
    main()

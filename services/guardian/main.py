"""
main.py — Punto de Entrada del Agente Guardián
================================================
Proceso Python asyncio que:
  1. Carga la configuración y valida las variables de entorno.
  2. Instancia el EmbeddingService (singleton, carga el modelo UNA sola vez).
  3. Inicializa el VectorRepository con pool de conexiones psycopg2.
  4. Conecta a NATS y se suscribe al subject ``node.creation.requested``.
  5. Procesa cada evento de creación de nodo aplicando el flujo completo del
     PRD Sección 6.2 (embed → buscar similares → decidir → auditar → responder).
  6. Maneja señales SIGTERM/SIGINT para un shutdown elegante.

REGLA ESTRICTA (agente_guardian.md):
  - El modelo de embeddings se instancia ANTES de entrar al event loop.
  - El pool de conexiones se instancia ANTES de entrar al event loop.
  - Cualquier error en el handler responde con approve+needs_review para no
    bloquear al usuario (degradación elegante).
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
from nats.aio.client import Client as NATSClient
from nats.aio.msg import Msg

from config import settings
from decision import evaluate_similarity
from embeddings import EmbeddingService
from vector_repo import VectorRepository

# ─── Logging ──────────────────────────────────────────────────────────────────
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    datefmt="%Y-%m-%dT%H:%M:%S",
    stream=sys.stdout,
)
logger = logging.getLogger("guardian")

# ─── Sujetos NATS ─────────────────────────────────────────────────────────────
NATS_SUBJECT_IN = "node.creation.requested"

# ─── Globals (inicializados en main(), usados en el handler) ──────────────────
_embedding_service: EmbeddingService
_vector_repo: VectorRepository


# ─────────────────────────────────────────────────────────────────────────────
# Handler NATS
# ─────────────────────────────────────────────────────────────────────────────

async def guardian_handler(msg: Msg) -> None:
    """
    Callback asíncrono para cada mensaje recibido en ``node.creation.requested``.

    Implementa el flujo completo del PRD Sección 6.2:
      a. Deserializar el payload JSON.
      b. Concatenar title + " " + description.
      c. Generar el embedding con EmbeddingService.generate().
      d. Consultar pgvector con timeout de 2 s (PRD R3).
      e. Evaluar la decisión con evaluate_similarity (función pura).
      f. Registrar auditoría en agent_audit_log.
      g. Responder al reply subject si existe (NATS Request-Reply).

    En caso de error en cualquier paso, responde con
    ``{"decision": "approve", "needs_review": True, "error": "<msg>"}``
    para no bloquear al usuario (degradación elegante).

    Args:
        msg: Mensaje NATS recibido con datos serializados en JSON.
    """
    node_id: str = "<desconocido>"
    event: dict[str, Any] = {}

    try:
        # ── a) Deserializar payload ───────────────────────────────────────────
        raw = msg.data.decode("utf-8")
        event = json.loads(raw)
        node_id = event.get("node_id", "<desconocido>")
        logger.info("Evento recibido: node_id=%s", node_id)

        # ── b) Construir texto a vectorizar ───────────────────────────────────
        title: str = event.get("title", "")
        description: str = event.get("description", "")
        text_to_embed: str = f"{title} {description}".strip()

        if not text_to_embed:
            raise ValueError(
                "El evento no contiene 'title' ni 'description' válidos."
            )

        # ── c) Generar embedding ──────────────────────────────────────────────
        embedding: list[float] = _embedding_service.generate(text_to_embed)
        logger.debug("Embedding generado para node_id=%s (dim=%d).", node_id, len(embedding))

        # ── d) Consultar pgvector con timeout de 2 s (PRD R3) ─────────────────
        timeout_occurred = False
        similar_nodes: list[dict[str, Any]] = []
        try:
            similar_nodes = await asyncio.wait_for(
                asyncio.get_event_loop().run_in_executor(
                    None,
                    lambda: _vector_repo.search_similar_nodes(
                        embedding=embedding,
                        threshold=settings.similarity_threshold_suggest,
                        limit=settings.max_similar_candidates,
                    ),
                ),
                timeout=2.0,
            )
            logger.info(
                "Búsqueda pgvector completada: %d candidatos para node_id=%s.",
                len(similar_nodes),
                node_id,
            )
        except asyncio.TimeoutError:
            logger.warning(
                "Timeout en búsqueda pgvector para node_id=%s. "
                "Se aplica degradación elegante (PRD R3).",
                node_id,
            )
            timeout_occurred = True

        # ── e) Evaluar decisión (función pura, sin efectos secundarios) ────────
        decision: dict[str, Any] = evaluate_similarity(
            similar_nodes=similar_nodes,
            config=settings,
            timeout=timeout_occurred,
        )
        logger.info(
            "Decisión para node_id=%s: %s", node_id, decision.get("decision")
        )

        # ── f) Registrar auditoría ─────────────────────────────────────────────
        max_similarity = (
            float(similar_nodes[0]["similarity"]) if similar_nodes else 0.0
        )
        _audit_safe(
            agent_type="guardian",
            action=decision["decision"],
            input_data={
                "node_id":     node_id,
                "title":       title,
                "description": description,
                "owner_id":    event.get("owner_id"),
                "requested_at": event.get("requested_at"),
            },
            output_data=decision,
            confidence=max_similarity,
            node_id=node_id,
        )

        # ── g) Responder al reply subject si existe ────────────────────────────
        if msg.reply:
            await msg.respond(json.dumps(decision).encode("utf-8"))
            logger.debug("Respuesta enviada al reply subject '%s'.", msg.reply)

    except Exception as exc:
        logger.exception(
            "Error inesperado procesando node_id=%s: %s", node_id, exc
        )
        error_decision: dict[str, Any] = {
            "decision": "approve",
            "needs_review": True,
            "error": str(exc),
        }

        # Intentar registrar el error en auditoría (best-effort)
        _audit_safe(
            agent_type="guardian",
            action="approve",
            input_data=event or {"raw": msg.data.decode("utf-8", errors="replace")},
            output_data=error_decision,
            confidence=0.0,
            node_id=node_id,
        )

        # Responder con aprobación degradada para no bloquear al usuario
        if msg.reply:
            try:
                await msg.respond(json.dumps(error_decision).encode("utf-8"))
            except Exception as reply_exc:
                logger.error("No se pudo enviar respuesta de error: %s", reply_exc)


def _audit_safe(
    agent_type: str,
    action: str,
    input_data: dict[str, Any],
    output_data: dict[str, Any],
    confidence: float,
    node_id: str,
) -> None:
    """
    Llama a VectorRepository.log_audit de forma segura (best-effort).

    Si el registro de auditoría falla, solo se registra en el log local
    sin propagar la excepción (para no romper el flujo principal).

    Args:
        agent_type: Tipo de agente (ej. "guardian").
        action: Acción tomada (ej. "approve", "block", "suggest").
        input_data: Datos de entrada en formato dict.
        output_data: Datos de salida en formato dict.
        confidence: Confianza/similitud máxima (0.0 – 1.0).
        node_id: UUID del nodo evaluado.
    """
    try:
        _vector_repo.log_audit(
            agent_type=agent_type,
            action=action,
            input_data=input_data,
            output_data=output_data,
            confidence=confidence,
            node_id=node_id,
        )
    except Exception as exc:
        logger.error(
            "No se pudo registrar auditoría para node_id=%s: %s", node_id, exc
        )


# ─────────────────────────────────────────────────────────────────────────────
# Punto de entrada principal
# ─────────────────────────────────────────────────────────────────────────────

async def _async_main() -> None:
    """
    Función principal asíncrona del Agente Guardián.

    Secuencia de arranque:
      1. Instanciar EmbeddingService (carga modelo sentence-transformers).
      2. Instanciar VectorRepository (crea pool de conexiones psycopg2).
      3. Conectar a NATS con reconexión automática.
      4. Suscribirse a ``node.creation.requested``.
      5. Correr indefinidamente hasta recibir señal de parada.
      6. Shutdown elegante: cerrar suscripción → NATS → pool psycopg2.
    """
    global _embedding_service, _vector_repo

    logger.info("════════════════════════════════════════════")
    logger.info("   Agente Guardián — Iniciando servicio     ")
    logger.info("════════════════════════════════════════════")

    # ── 1) Cargar modelo de embeddings (singleton) ─────────────────────────
    logger.info("Cargando modelo de embeddings '%s'...", settings.embedding_model)
    _embedding_service = EmbeddingService(model_name=settings.embedding_model)

    # ── 2) Inicializar pool de conexiones PostgreSQL ────────────────────────
    logger.info("Conectando a PostgreSQL...")
    _vector_repo = VectorRepository(database_url=settings.database_url)

    # ── 3) Conectar a NATS ─────────────────────────────────────────────────
    logger.info("Conectando a NATS en %s ...", settings.nats_url)
    nc: NATSClient = await nats.connect(
        servers=[settings.nats_url],
        reconnect_time_wait=2,
        max_reconnect_attempts=-1,
        error_cb=_nats_error_cb,
        reconnected_cb=_nats_reconnected_cb,
        disconnected_cb=_nats_disconnected_cb,
        closed_cb=_nats_closed_cb,
    )
    logger.info("Conectado a NATS. URL: %s", nc.connected_url)

    # ── 4) Suscribirse al subject de entrada ───────────────────────────────
    subscription = await nc.subscribe(NATS_SUBJECT_IN, cb=guardian_handler)
    logger.info("Suscrito al subject '%s'. Esperando eventos...", NATS_SUBJECT_IN)

    # ── 5) Esperar hasta recibir señal de parada ───────────────────────────
    stop_event = asyncio.Event()

    def _handle_signal() -> None:
        logger.info("Señal de parada recibida. Iniciando shutdown...")
        stop_event.set()

    loop = asyncio.get_event_loop()
    for sig in (signal.SIGTERM, signal.SIGINT):
        try:
            loop.add_signal_handler(sig, _handle_signal)
        except (NotImplementedError, AttributeError):
            # Windows no soporta add_signal_handler para todos los signals
            signal.signal(sig, lambda s, f: _handle_signal())

    await stop_event.wait()

    # ── 6) Shutdown elegante ───────────────────────────────────────────────
    logger.info("Cancelando suscripción NATS...")
    await subscription.unsubscribe()

    logger.info("Drenando y cerrando conexión NATS...")
    await nc.drain()
    await nc.close()

    logger.info("Cerrando pool de conexiones PostgreSQL...")
    _vector_repo.close()

    logger.info("Agente Guardián detenido limpiamente.")


# ─── Callbacks NATS ──────────────────────────────────────────────────────────

async def _nats_error_cb(exc: Exception) -> None:
    """Callback de error de conexión NATS."""
    logger.error("Error en conexión NATS: %s", exc)


async def _nats_reconnected_cb() -> None:
    """Callback de reconexión exitosa a NATS."""
    logger.info("Reconectado a NATS exitosamente.")


async def _nats_disconnected_cb() -> None:
    """Callback de desconexión de NATS."""
    logger.warning("Desconectado de NATS. Intentando reconexión...")


async def _nats_closed_cb() -> None:
    """Callback de cierre definitivo de la conexión NATS."""
    logger.info("Conexión NATS cerrada.")


# ─────────────────────────────────────────────────────────────────────────────

if __name__ == "__main__":
    try:
        asyncio.run(_async_main())
    except KeyboardInterrupt:
        logger.info("Interrupción por teclado. Saliendo...")

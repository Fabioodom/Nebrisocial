"""
message_repo.py — Repositorio de acceso a datos para el Agente Cronista.

Implementa las operaciones de base de datos para:
  - Obtener nodos activos con actividad de chat reciente (sin resumen previo).
  - Leer los mensajes de chat de un nodo en las últimas N horas.
  - Guardar el hilo de resumen IA en la tabla threads.
  - Registrar acciones en agent_audit_log.

Usa psycopg2 con reconexión automática. La privacidad de los usuarios se
garantiza a nivel de repositorio: get_messages_for_node nunca devuelve
el user_id real, solo el username anonimizado en el chunker.
"""
from __future__ import annotations

import json
import logging
from typing import Any

import psycopg2
import psycopg2.extras

log = logging.getLogger(__name__)


class MessageRepository:
    """Gestiona las operaciones de base de datos del Agente Cronista."""

    def __init__(self, database_url: str) -> None:
        """
        Inicializa el repositorio y abre la conexión a PostgreSQL.

        Args:
            database_url: Cadena de conexión DSN (postgresql://...).
        """
        self._database_url = database_url
        self._conn: psycopg2.extensions.connection | None = None
        self._connect()

    # ── Conexión ──────────────────────────────────────────────────────────────

    def _connect(self) -> None:
        """Establece o restablece la conexión a PostgreSQL."""
        try:
            self._conn = psycopg2.connect(
                self._database_url,
                cursor_factory=psycopg2.extras.RealDictCursor,
            )
            self._conn.autocommit = False
            log.info("MessageRepository: conexión a PostgreSQL establecida.")
        except psycopg2.OperationalError as exc:
            log.error("MessageRepository: no se pudo conectar a PostgreSQL: %s", exc)
            raise

    def _ensure_connection(self) -> None:
        """Reconecta si la conexión se ha cerrado o está en estado de error."""
        if self._conn is None or self._conn.closed:
            log.warning("MessageRepository: reconectando a PostgreSQL...")
            self._connect()
        else:
            if self._conn.status == psycopg2.extensions.STATUS_IN_TRANSACTION:
                self._conn.rollback()

    # ── Operaciones de lectura ────────────────────────────────────────────────

    def get_active_nodes_with_messages(self, min_messages: int) -> list[dict[str, Any]]:
        """
        Devuelve los nodos con más de `min_messages` mensajes en las últimas 24h,
        excluyendo los que ya tienen un resumen IA generado en ese período.

        Args:
            min_messages: Umbral mínimo de mensajes para considerar un nodo activo.

        Returns:
            Lista de dicts con campos: id, title.
        """
        self._ensure_connection()
        sql = """
            SELECT n.id, n.title
            FROM nodes n
            JOIN chat_messages cm ON cm.node_id = n.id
            WHERE cm.created_at >= NOW() - INTERVAL '24 hours'
              AND n.status = 'active'
              AND NOT EXISTS (
                  SELECT 1 FROM threads t
                  WHERE t.node_id = n.id
                    AND t.is_ai_generated = TRUE
                    AND t.created_at >= NOW() - INTERVAL '24 hours'
              )
            GROUP BY n.id, n.title
            HAVING COUNT(cm.id) > %s
            ORDER BY COUNT(cm.id) DESC
        """
        try:
            with self._conn.cursor() as cur:  # type: ignore[union-attr]
                cur.execute(sql, (min_messages,))
                rows = cur.fetchall()
                return [dict(row) for row in rows]
        except psycopg2.Error as exc:
            self._conn.rollback()  # type: ignore[union-attr]
            log.error("get_active_nodes_with_messages error: %s", exc)
            raise

    def get_messages_for_node(
        self, node_id: str, since_hours: int = 24
    ) -> list[dict[str, Any]]:
        """
        Devuelve los mensajes de un nodo en las últimas `since_hours` horas.

        Hace JOIN con users para obtener el username, pero el chunker es
        responsable de anonimizarlo (PRD R3). Excluye mensajes reportados.

        Args:
            node_id: UUID del nodo.
            since_hours: Ventana temporal en horas hacia atrás.

        Returns:
            Lista de dicts con: content, username, created_at.
            Ordenados cronológicamente (ASC).
        """
        self._ensure_connection()
        sql = """
            SELECT
                cm.content,
                u.username,
                cm.created_at
            FROM chat_messages cm
            LEFT JOIN users u ON u.id = cm.user_id
            WHERE cm.node_id = %s
              AND cm.created_at >= NOW() - INTERVAL '1 hour' * %s
            ORDER BY cm.created_at ASC
        """
        try:
            with self._conn.cursor() as cur:  # type: ignore[union-attr]
                cur.execute(sql, (node_id, since_hours))
                rows = cur.fetchall()
                return [dict(row) for row in rows]
        except psycopg2.Error as exc:
            self._conn.rollback()  # type: ignore[union-attr]
            log.error(
                "get_messages_for_node error para node_id=%s: %s", node_id, exc
            )
            raise

    # ── Operaciones de escritura ──────────────────────────────────────────────

    def save_summary_thread(
        self, node_id: str, title: str, body: str
    ) -> str:
        """
        Inserta el resumen IA como un nuevo hilo en la tabla threads.

        El hilo se publica con is_ai_generated=TRUE, pinned=TRUE y
        author_id=NULL (sin autor humano, conforme al PRD).

        Args:
            node_id: UUID del nodo al que pertenece el resumen.
            title: Título del hilo (ej. "[Resumen IA] Mi Nodo — 2026-05-25").
            body: Cuerpo del resumen en Markdown.

        Returns:
            thread_id (UUID) del hilo creado.

        Raises:
            psycopg2.Error: Si la inserción falla.
        """
        self._ensure_connection()
        sql = """
            INSERT INTO threads
                (node_id, author_id, title, body, is_ai_generated, pinned)
            VALUES (%s, NULL, %s, %s, TRUE, TRUE)
            RETURNING id
        """
        try:
            with self._conn.cursor() as cur:  # type: ignore[union-attr]
                cur.execute(sql, (node_id, title, body))
                row = cur.fetchone()
                thread_id: str = str(row["id"])  # type: ignore[index]
            self._conn.commit()  # type: ignore[union-attr]
            log.info(
                "save_summary_thread: hilo %s creado para node_id=%s",
                thread_id,
                node_id,
            )
            return thread_id
        except psycopg2.Error as exc:
            self._conn.rollback()  # type: ignore[union-attr]
            log.error(
                "save_summary_thread error para node_id=%s: %s", node_id, exc
            )
            raise

    def log_audit(
        self,
        agent_type: str,
        action: str,
        input_data: dict[str, Any],
        output_data: dict[str, Any],
        node_id: str,
    ) -> None:
        """
        Inserta un registro en la tabla agent_audit_log.

        El fallo de auditoría NO interrumpe el flujo principal.

        Args:
            agent_type: Tipo de agente (ej. "cronista").
            action: Acción realizada (ej. "summary_created" / "summary_failed").
            input_data: Datos de entrada del agente (JSONB).
            output_data: Datos de salida / resultado (JSONB).
            node_id: UUID del nodo relacionado.
        """
        self._ensure_connection()
        sql = """
            INSERT INTO agent_audit_log
                (agent_type, action, input_data, output_data, node_id)
            VALUES (%s, %s, %s::jsonb, %s::jsonb, %s)
        """
        try:
            with self._conn.cursor() as cur:  # type: ignore[union-attr]
                cur.execute(
                    sql,
                    (
                        agent_type,
                        action,
                        json.dumps(input_data),
                        json.dumps(output_data),
                        node_id,
                    ),
                )
            self._conn.commit()  # type: ignore[union-attr]
            log.debug(
                "log_audit: registrado [%s/%s] para nodo %s",
                agent_type,
                action,
                node_id,
            )
        except psycopg2.Error as exc:
            self._conn.rollback()  # type: ignore[union-attr]
            log.error("log_audit error para node_id=%s: %s", node_id, exc)
            log.warning("El registro de auditoría falló; continuando de todos modos.")

    # ── Limpieza ──────────────────────────────────────────────────────────────

    def close(self) -> None:
        """Cierra la conexión a PostgreSQL."""
        if self._conn and not self._conn.closed:
            self._conn.close()
            log.info("MessageRepository: conexión a PostgreSQL cerrada.")

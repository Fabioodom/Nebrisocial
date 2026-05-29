"""
node_repo.py — Repositorio de acceso a datos para la tabla `nodes`.

Implementa las operaciones de base de datos necesarias para el Agente Curador:
  - Leer el nodo actual (con su metadata JSONB existente).
  - Actualizar el campo metadata usando el operador || de PostgreSQL (no
    sobrescribe campos que el usuario haya editado manualmente).
  - Registrar acciones en la tabla agent_audit_log.

Usa psycopg2 con conexión directa (sin pool) dado el volumen esperado en
la Fase 3 del MVP. Migrar a psycopg3 + asyncpg en producción si se requiere
mayor concurrencia.
"""
from __future__ import annotations

import json
import logging
from typing import Any

import psycopg2
import psycopg2.extras

log = logging.getLogger(__name__)


class NodeRepository:
    """Gestiona las operaciones de base de datos relacionadas con nodos."""

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
            log.info("NodeRepository: conexión a PostgreSQL establecida.")
        except psycopg2.OperationalError as exc:
            log.error("NodeRepository: no se pudo conectar a PostgreSQL: %s", exc)
            raise

    def _ensure_connection(self) -> None:
        """Reconecta si la conexión se ha cerrado o está en estado de error."""
        if self._conn is None or self._conn.closed:
            log.warning("NodeRepository: reconectando a PostgreSQL...")
            self._connect()
        else:
            # Detecta transacciones abortadas y las revierte antes de continuar
            if self._conn.status == psycopg2.extensions.STATUS_IN_TRANSACTION:
                self._conn.rollback()

    # ── Operaciones de lectura ────────────────────────────────────────────────

    def get_node(self, node_id: str) -> dict[str, Any]:
        """
        Recupera title, description, category y metadata de un nodo.

        Args:
            node_id: UUID del nodo.

        Returns:
            Diccionario con los campos del nodo. metadata es un dict JSONB.

        Raises:
            ValueError: Si el nodo no existe.
            psycopg2.Error: Si hay un error de base de datos.
        """
        self._ensure_connection()
        sql = """
            SELECT title, description, category, metadata
            FROM nodes
            WHERE id = %s
        """
        try:
            with self._conn.cursor() as cur:  # type: ignore[union-attr]
                cur.execute(sql, (node_id,))
                row = cur.fetchone()
        except psycopg2.Error as exc:
            self._conn.rollback()  # type: ignore[union-attr]
            log.error("get_node error para node_id=%s: %s", node_id, exc)
            raise

        if row is None:
            raise ValueError(f"Nodo con id={node_id} no encontrado en la base de datos.")

        result = dict(row)
        # psycopg2 ya deserializa JSONB a dict, pero aseguramos el tipo
        if isinstance(result.get("metadata"), str):
            result["metadata"] = json.loads(result["metadata"])
        if result["metadata"] is None:
            result["metadata"] = {}

        return result

    # ── Operaciones de escritura ──────────────────────────────────────────────

    def update_node_metadata(self, node_id: str, new_metadata: dict[str, Any]) -> None:
        """
        Actualiza el campo metadata del nodo usando el operador JSONB ||.

        El operador || fusiona (merge) el JSONB existente con new_metadata.
        Las claves que ya existen en metadata NO se sobrescriben si new_metadata
        no las incluye; las claves nuevas se agregan.

        Args:
            node_id: UUID del nodo a actualizar.
            new_metadata: Diccionario con los nuevos campos a agregar/actualizar.

        Raises:
            psycopg2.Error: Si la operación de escritura falla.
        """
        self._ensure_connection()
        # PRD R2: operador || garantiza no sobrescribir campos existentes que
        # el usuario editó manualmente (la verificación previa en curator_agent.py
        # añade una capa extra de protección).
        sql = """
            UPDATE nodes
            SET metadata = metadata || %s::jsonb
            WHERE id = %s
        """
        try:
            with self._conn.cursor() as cur:  # type: ignore[union-attr]
                cur.execute(sql, (json.dumps(new_metadata), node_id))
            self._conn.commit()  # type: ignore[union-attr]
            log.info(
                "update_node_metadata: nodo %s actualizado con campos %s",
                node_id,
                list(new_metadata.keys()),
            )
        except psycopg2.Error as exc:
            self._conn.rollback()  # type: ignore[union-attr]
            log.error(
                "update_node_metadata error para node_id=%s: %s", node_id, exc
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

        Args:
            agent_type: Tipo de agente (ej. "curator").
            action: Acción realizada (ej. "metadata_enriched").
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
            log.error(
                "log_audit error para node_id=%s: %s", node_id, exc
            )
            # El fallo de auditoría NO debe interrumpir el flujo principal
            log.warning("El registro de auditoría falló; continuando de todos modos.")

    # ── Limpieza ──────────────────────────────────────────────────────────────

    def close(self) -> None:
        """Cierra la conexión a PostgreSQL."""
        if self._conn and not self._conn.closed:
            self._conn.close()
            log.info("NodeRepository: conexión a PostgreSQL cerrada.")

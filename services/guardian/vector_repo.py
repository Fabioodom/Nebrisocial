"""
vector_repo.py — Repositorio pgvector para el Agente Guardián
==============================================================
Gestiona toda la interacción con PostgreSQL:
  - Búsqueda de nodos similares via pgvector (similitud coseno).
  - Persistencia de embeddings en node_embeddings.
  - Registro de auditoría en agent_audit_log.

Usa un ThreadedConnectionPool de psycopg2 para no crear una nueva
conexión por cada mensaje NATS recibido.
"""

from __future__ import annotations

import json
import logging
from typing import Any

import psycopg2
import psycopg2.extras
from psycopg2 import pool as pg_pool

logger = logging.getLogger(__name__)

# Consulta pgvector de similitud coseno sobre node_embeddings
_SEARCH_SIMILAR_QUERY = """
    SELECT
        ne.node_id::text,
        n.title,
        n.slug,
        1 - (ne.embedding <=> %s::vector) AS similarity
    FROM node_embeddings ne
    JOIN nodes n ON ne.node_id = n.id
    WHERE 1 - (ne.embedding <=> %s::vector) > %s
    ORDER BY similarity DESC
    LIMIT %s;
"""

_SAVE_EMBEDDING_QUERY = """
    INSERT INTO node_embeddings (node_id, embedding, model_version)
    VALUES (%s, %s::vector, %s)
    ON CONFLICT (node_id) DO UPDATE
        SET embedding     = EXCLUDED.embedding,
            model_version = EXCLUDED.model_version,
            created_at    = NOW();
"""

_LOG_AUDIT_QUERY = """
    INSERT INTO agent_audit_log
        (agent_type, action, input_data, output_data, confidence, node_id)
    VALUES (%s, %s, %s::jsonb, %s::jsonb, %s, %s::uuid);
"""


class VectorRepository:
    """
    Repositorio de acceso a datos para el Agente Guardián.

    Gestiona un pool de conexiones psycopg2 que se comparte entre el
    event loop asyncio y los threads de psycopg2 (ThreadedConnectionPool).

    Args:
        database_url: DSN de PostgreSQL (ej. postgresql://user:pass@host/db).
        minconn: Número mínimo de conexiones en el pool.
        maxconn: Número máximo de conexiones en el pool.
    """

    def __init__(
        self,
        database_url: str,
        minconn: int = 1,
        maxconn: int = 10,
    ) -> None:
        """
        Inicializa el pool de conexiones psycopg2.

        Args:
            database_url: DSN de PostgreSQL.
            minconn: Conexiones mínimas abiertas al arrancar.
            maxconn: Conexiones máximas permitidas en el pool.

        Raises:
            psycopg2.OperationalError: Si no puede conectar a la base de datos.
        """
        logger.info("Inicializando pool de conexiones PostgreSQL...")
        self._pool = pg_pool.ThreadedConnectionPool(
            minconn=minconn,
            maxconn=maxconn,
            dsn=database_url,
        )
        logger.info("Pool PostgreSQL listo (min=%d, max=%d).", minconn, maxconn)

    # ──────────────────────────────────────────────────────────────────────────
    # Métodos públicos
    # ──────────────────────────────────────────────────────────────────────────

    def search_similar_nodes(
        self,
        embedding: list[float],
        threshold: float,
        limit: int,
    ) -> list[dict[str, Any]]:
        """
        Busca nodos semánticamente similares al embedding dado.

        Ejecuta una consulta pgvector de similitud coseno sobre node_embeddings.
        Solo se devuelven candidatos cuya similitud supere `threshold`.

        Args:
            embedding: Vector de 384 floats a comparar.
            threshold: Umbral mínimo de similitud (0.0 – 1.0) para filtrar.
            limit: Número máximo de resultados a devolver.

        Returns:
            Lista de dicts con las claves:
                - node_id (str): UUID del nodo encontrado.
                - title (str): Título del nodo.
                - slug (str): Slug del nodo.
                - similarity (float): Similitud coseno (0.0 – 1.0).

        Raises:
            psycopg2.DatabaseError: Si la consulta SQL falla.
        """
        embedding_str = self._vector_to_pg_str(embedding)
        conn = self._pool.getconn()
        try:
            with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
                cur.execute(
                    _SEARCH_SIMILAR_QUERY,
                    (embedding_str, embedding_str, threshold, limit),
                )
                rows = cur.fetchall()
            conn.commit()
            return [dict(row) for row in rows]
        except Exception:
            conn.rollback()
            raise
        finally:
            self._pool.putconn(conn)

    def save_embedding(
        self,
        node_id: str,
        embedding: list[float],
        model_version: str = "all-MiniLM-L6-v2",
    ) -> None:
        """
        Inserta o actualiza el embedding de un Nodo en node_embeddings.

        Usa INSERT … ON CONFLICT para hacer upsert seguro.

        Args:
            node_id: UUID del nodo como cadena de texto.
            embedding: Vector de 384 floats a persistir.
            model_version: Nombre del modelo usado para generar el embedding.

        Raises:
            psycopg2.DatabaseError: Si la escritura en BD falla.
        """
        embedding_str = self._vector_to_pg_str(embedding)
        conn = self._pool.getconn()
        try:
            with conn.cursor() as cur:
                cur.execute(_SAVE_EMBEDDING_QUERY, (node_id, embedding_str, model_version))
            conn.commit()
            logger.debug("Embedding guardado para node_id=%s.", node_id)
        except Exception:
            conn.rollback()
            raise
        finally:
            self._pool.putconn(conn)

    def log_audit(
        self,
        agent_type: str,
        action: str,
        input_data: dict[str, Any],
        output_data: dict[str, Any],
        confidence: float,
        node_id: str,
    ) -> None:
        """
        Registra una entrada en agent_audit_log.

        Args:
            agent_type: Tipo de agente que emite el log (ej. "guardian").
            action: Decisión tomada (ej. "approve", "block", "suggest").
            input_data: Payload de entrada serializable como JSONB.
            output_data: Decisión de salida serializable como JSONB.
            confidence: Valor de similitud máxima encontrada (0.0 si ninguna).
            node_id: UUID del nodo que se estaba evaluando.

        Raises:
            psycopg2.DatabaseError: Si la escritura en BD falla.
        """
        conn = self._pool.getconn()
        try:
            with conn.cursor() as cur:
                cur.execute(
                    _LOG_AUDIT_QUERY,
                    (
                        agent_type,
                        action,
                        json.dumps(input_data, default=str),
                        json.dumps(output_data, default=str),
                        confidence,
                        node_id,
                    ),
                )
            conn.commit()
            logger.debug(
                "Auditoría registrada: agent=%s action=%s node_id=%s confidence=%.4f.",
                agent_type,
                action,
                node_id,
                confidence,
            )
        except Exception:
            conn.rollback()
            raise
        finally:
            self._pool.putconn(conn)

    def close(self) -> None:
        """
        Cierra todas las conexiones del pool de forma ordenada.

        Debe llamarse durante el shutdown del proceso (SIGTERM/SIGINT).
        """
        logger.info("Cerrando pool de conexiones PostgreSQL...")
        self._pool.closeall()

    # ──────────────────────────────────────────────────────────────────────────
    # Helpers privados
    # ──────────────────────────────────────────────────────────────────────────

    @staticmethod
    def _vector_to_pg_str(embedding: list[float]) -> str:
        """
        Convierte una lista de floats al formato literal de vector de pgvector.

        Args:
            embedding: Lista de valores float.

        Returns:
            Cadena con formato '[v1,v2,...,vN]' aceptada por pgvector.
        """
        return "[" + ",".join(str(v) for v in embedding) + "]"

"""
embeddings.py — Servicio de Generación de Embeddings
======================================================
Encapsula el modelo sentence-transformers/all-MiniLM-L6-v2 con patrón
singleton: el modelo se carga UNA SOLA VEZ al instanciar la clase y
se reutiliza para todas las llamadas a generate().

Dimensión de salida: 384 floats.
Latencia estimada en CPU: < 50 ms por inferencia.
"""

from __future__ import annotations

import logging
from typing import ClassVar

from sentence_transformers import SentenceTransformer

logger = logging.getLogger(__name__)


class EmbeddingService:
    """
    Servicio singleton para generación de embeddings de texto.

    El modelo se carga en el constructor y se mantiene en memoria durante
    toda la vida del proceso. No volver a instanciar esta clase más de una
    vez por proceso (ver main.py).

    Attributes:
        _model: Instancia del modelo SentenceTransformer cargado.
        EMBEDDING_DIM: Dimensión de los vectores generados (384).
    """

    EMBEDDING_DIM: ClassVar[int] = 384

    def __init__(self, model_name: str = "all-MiniLM-L6-v2") -> None:
        """
        Inicializa y carga el modelo sentence-transformers en memoria.

        Args:
            model_name: Nombre del modelo HuggingFace a cargar.
                        Por defecto 'all-MiniLM-L6-v2' (384 dims, rápido en CPU).

        Raises:
            RuntimeError: Si el modelo no puede descargarse o cargarse.
        """
        logger.info("Cargando modelo de embeddings: %s ...", model_name)
        try:
            self._model = SentenceTransformer(model_name)
        except Exception as exc:
            raise RuntimeError(
                f"No se pudo cargar el modelo de embeddings '{model_name}': {exc}"
            ) from exc
        logger.info(
            "Modelo '%s' cargado. Dimensión de salida: %d.",
            model_name,
            self.EMBEDDING_DIM,
        )

    def generate(self, text: str) -> list[float]:
        """
        Genera el vector de embeddings para el texto dado.

        El texto de entrada es normalmente la concatenación de:
        ``event["title"] + " " + event["description"]``

        Args:
            text: Cadena de texto a vectorizar.

        Returns:
            Lista de 384 floats representando el embedding semántico.

        Raises:
            ValueError: Si el texto de entrada está vacío.
            RuntimeError: Si la inferencia del modelo falla.
        """
        if not text or not text.strip():
            raise ValueError("El texto para generar embedding no puede estar vacío.")

        try:
            # encode() devuelve un numpy.ndarray; convertimos a lista de floats
            vector: list[float] = self._model.encode(
                text,
                convert_to_numpy=True,
                normalize_embeddings=True,  # norma L2 → similitud coseno == producto punto
            ).tolist()
        except Exception as exc:
            raise RuntimeError(
                f"Error durante la inferencia del modelo de embeddings: {exc}"
            ) from exc

        if len(vector) != self.EMBEDDING_DIM:
            raise RuntimeError(
                f"Dimensión inesperada del embedding: se esperaban {self.EMBEDDING_DIM} "
                f"dims, se obtuvieron {len(vector)}."
            )

        return vector

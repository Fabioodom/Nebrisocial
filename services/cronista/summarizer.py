"""
summarizer.py — Síntesis LLM con LangChain para el Agente Cronista.

Implementa la estrategia Map-Reduce del PRD (Sección 6.3):
  - MAP: resume brevemente cada chunk individual.
  - REDUCE: sintetiza todos los resúmenes parciales en un resumen estructurado
    con las secciones ## Temas | ## Destacados | ## Recursos.

Compatible con OpenAI (langchain-openai) y Google Gemini (langchain-google-genai)
según la variable LLM_PROVIDER.
"""
from __future__ import annotations

import logging
from typing import TYPE_CHECKING

from langchain_core.output_parsers import StrOutputParser
from langchain_core.prompts import ChatPromptTemplate

if TYPE_CHECKING:
    from langchain_core.language_models import BaseChatModel

log = logging.getLogger(__name__)

# ── Plantillas de prompt ───────────────────────────────────────────────────────

_MAP_TEMPLATE = """\
Eres un asistente que ayuda a resumir conversaciones de comunidades online.
Resume brevemente los temas principales de este fragmento de conversación del Nodo '{node_title}'.
Sé conciso. No incluyas nombres de usuario. Máximo 100 palabras.

Fragmento:
{chunk}

Resumen del fragmento:"""

_REDUCE_TEMPLATE = """\
Eres un asistente experto en síntesis de conversaciones comunitarias.
A continuación tienes los resúmenes parciales de la conversación de hoy en el Nodo '{node_title}'.

Resúmenes parciales:
{partial_summaries}

Basándote en estos resúmenes parciales, genera un Resumen Diario estructurado en español.
El resumen NO debe superar {max_words} palabras en total.
No menciones nombres de usuario reales; usa "Un participante" o "Un miembro".
Usa el siguiente formato Markdown exacto:

## 🗂️ Temas del Día
[Lista los 3-5 temas principales que se discutieron hoy, con una breve descripción de cada uno.]

## ⭐ Destacados
[Menciona 2-4 momentos, ideas o contribuciones especialmente relevantes o interesantes.]

## 🔗 Recursos
[Lista los enlaces, herramientas, libros, juegos u otros recursos mencionados. Si no se mencionó ninguno, escribe "No se compartieron recursos hoy."]

Resumen Diario:"""


def _build_llm(provider: str, api_key: str, model: str) -> "BaseChatModel":
    """
    Instancia el LLM correcto según el proveedor configurado.

    Args:
        provider: "openai" o "google".
        api_key: Clave API del proveedor.
        model: Nombre del modelo (ej. "gpt-4o-mini", "gemini-1.5-flash").

    Returns:
        Instancia de BaseChatModel lista para usar.

    Raises:
        ValueError: Si el proveedor no está soportado.
        ImportError: Si el paquete del proveedor no está instalado.
    """
    if provider == "openai":
        from langchain_openai import ChatOpenAI  # noqa: PLC0415
        return ChatOpenAI(
            model=model,
            api_key=api_key,  # type: ignore[arg-type]
            temperature=0.3,
            max_tokens=2048,
        )
    elif provider == "google":
        from langchain_google_genai import ChatGoogleGenerativeAI  # noqa: PLC0415
        return ChatGoogleGenerativeAI(
            model=model,
            google_api_key=api_key,  # type: ignore[arg-type]
            temperature=0.3,
        )
    else:
        raise ValueError(
            f"LLM_PROVIDER '{provider}' no soportado. Usa 'openai' o 'google'."
        )


class Summarizer:
    """
    Sintetizador de conversaciones usando LangChain Map-Reduce.

    Attributes:
        provider: Proveedor LLM ("openai" o "google").
        model: Nombre del modelo.
        max_words: Límite de palabras del resumen final (PRD R2).
    """

    def __init__(
        self,
        provider: str,
        api_key: str,
        model: str,
        max_words: int = 800,
    ) -> None:
        """
        Inicializa el Summarizer con el LLM configurado.

        Args:
            provider: "openai" o "google".
            api_key: Clave API del proveedor.
            model: Nombre del modelo LLM.
            max_words: Máximo de palabras en el resumen final.
        """
        self._llm = _build_llm(provider, api_key, model)
        self._max_words = max_words
        self._provider = provider
        self._model = model

        # Cadenas LangChain reutilizables
        self._map_chain = (
            ChatPromptTemplate.from_template(_MAP_TEMPLATE)
            | self._llm
            | StrOutputParser()
        )
        self._reduce_chain = (
            ChatPromptTemplate.from_template(_REDUCE_TEMPLATE)
            | self._llm
            | StrOutputParser()
        )

        log.info(
            "Summarizer inicializado: provider=%s, model=%s, max_words=%d.",
            provider,
            model,
            max_words,
        )

    def summarize(self, node_title: str, chunks: list[str]) -> str:
        """
        Genera el resumen diario de un nodo aplicando Map-Reduce.

        Flujo:
          1. MAP: invoca el LLM una vez por chunk para obtener resúmenes parciales.
          2. REDUCE: combina los resúmenes parciales en un resumen estructurado final.
          3. Validación: el resumen final debe tener al menos 3 líneas no vacías
             y no superar `max_words` palabras.

        Args:
            node_title: Título del nodo (para personalizar el prompt).
            chunks: Lista de strings; cada uno es un bloque de mensajes formateados.

        Returns:
            Resumen diario en formato Markdown.

        Raises:
            ValueError: Si el resumen generado está vacío o tiene menos de 3 líneas.
            RuntimeError: Si el LLM falla durante la síntesis.
        """
        if not chunks:
            raise ValueError("No hay chunks para sintetizar.")

        log.info(
            "Summarizer.summarize: nodo='%s', %d chunk(s) a procesar.",
            node_title,
            len(chunks),
        )

        # ── Fase MAP ──────────────────────────────────────────────────────────
        partial_summaries: list[str] = []
        for idx, chunk in enumerate(chunks, start=1):
            log.debug("MAP chunk %d/%d para nodo='%s'...", idx, len(chunks), node_title)
            try:
                partial = self._map_chain.invoke(
                    {"node_title": node_title, "chunk": chunk}
                )
                partial_summaries.append(partial.strip())
            except Exception as exc:  # noqa: BLE001
                log.warning(
                    "MAP falló en chunk %d para nodo='%s': %s. Ignorando chunk.",
                    idx,
                    node_title,
                    exc,
                )

        if not partial_summaries:
            raise RuntimeError(
                f"Todos los chunks fallaron en la fase MAP para nodo='{node_title}'."
            )

        # ── Fase REDUCE ───────────────────────────────────────────────────────
        joined_partials = "\n\n---\n\n".join(
            f"Resumen {i + 1}:\n{s}" for i, s in enumerate(partial_summaries)
        )

        log.debug(
            "REDUCE: sintetizando %d resúmenes parciales para nodo='%s'...",
            len(partial_summaries),
            node_title,
        )
        try:
            final_summary = self._reduce_chain.invoke(
                {
                    "node_title": node_title,
                    "partial_summaries": joined_partials,
                    "max_words": self._max_words,
                }
            )
        except Exception as exc:  # noqa: BLE001
            raise RuntimeError(
                f"REDUCE falló para nodo='{node_title}': {exc}"
            ) from exc

        final_summary = final_summary.strip()

        # ── Validación ────────────────────────────────────────────────────────
        non_empty_lines = [ln for ln in final_summary.splitlines() if ln.strip()]
        if not final_summary:
            raise ValueError(
                f"El resumen generado está vacío para nodo='{node_title}'."
            )
        if len(non_empty_lines) < 3:
            raise ValueError(
                f"El resumen tiene solo {len(non_empty_lines)} líneas "
                f"(mínimo 3 requeridas) para nodo='{node_title}'."
            )

        # Truncar si supera el límite de palabras (PRD R2)
        word_count = len(final_summary.split())
        if word_count > self._max_words:
            log.warning(
                "Resumen supera el límite de %d palabras (%d). Truncando.",
                self._max_words,
                word_count,
            )
            words = final_summary.split()
            final_summary = " ".join(words[: self._max_words]) + "\n\n_[Resumen truncado por límite de longitud]_"

        log.info(
            "Summarizer.summarize completado para nodo='%s': %d palabras.",
            node_title,
            len(final_summary.split()),
        )
        return final_summary

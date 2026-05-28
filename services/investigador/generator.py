"""
generator.py — Servicio de generación de código UI con LLM.

Implementa el ciclo obligatorio de dos pasos: generate → review.
Compatible con Google Gemini (langchain-google-genai) y OpenAI (langchain-openai).
El LLM client se inicializa una sola vez (patrón singleton en main.py).
"""
from __future__ import annotations

import asyncio
import logging
import re
from dataclasses import dataclass, field
from pathlib import Path

from langchain_core.language_models.chat_models import BaseChatModel
from langchain_core.messages import HumanMessage, SystemMessage
from tenacity import (
    AsyncRetrying,
    RetryError,
    stop_after_attempt,
    wait_exponential,
)

from config import Settings
from prompts import (
    COMPONENT_GENERATION_PROMPT,
    PAGE_GENERATION_PROMPT,
    REVIEW_PROMPT,
    SYSTEM_PROMPT,
)

logger = logging.getLogger(__name__)


# ─────────────────────────────────────────────────────────────────────────────
# Data Models
# ─────────────────────────────────────────────────────────────────────────────

@dataclass
class GenerationResult:
    """Resultado de una generación de componente o página UI.

    Attributes:
        component_name: Nombre PascalCase del componente/página generado.
        templ_code: Código Go Templ completo y revisado.
        suggested_filename: Nombre de archivo sugerido en snake_case.templ.
        dependencies: Lista de atributos HTMX o componentes Nodal usados.
        review_notes: Notas del paso de revisión automática.
    """
    component_name: str
    templ_code: str
    suggested_filename: str
    dependencies: list[str] = field(default_factory=list)
    review_notes: str = ""


# ─────────────────────────────────────────────────────────────────────────────
# Helpers de extracción de texto
# ─────────────────────────────────────────────────────────────────────────────

def _extract_templ_code(text: str) -> str:
    """Extrae el bloque de código Go Templ de una respuesta del LLM.

    Busca el bloque entre marcadores ```templ ... ```.
    Si no encuentra el marcador, devuelve el texto completo (fallback).

    Args:
        text: Respuesta cruda del LLM.

    Returns:
        Código Go Templ extraído, sin los marcadores de código.
    """
    pattern = r"```templ\s*\n(.*?)```"
    match = re.search(pattern, text, re.DOTALL)
    if match:
        return match.group(1).strip()
    # Fallback: si el LLM devolvió código sin marcadores, limpiar y devolver
    logger.warning("No se encontró bloque ```templ en la respuesta. Usando texto completo.")
    return text.strip()


def _extract_component_name(templ_code: str) -> str:
    """Extrae el nombre del componente Templ del código generado.

    Busca el patrón `templ NombreFuncion(` en el código.

    Args:
        templ_code: Código Go Templ completo.

    Returns:
        Nombre del componente en PascalCase, o "GeneratedComponent" si no se encuentra.
    """
    match = re.search(r"templ\s+([A-Z][a-zA-Z0-9]+)\s*\(", templ_code)
    if match:
        return match.group(1)
    return "GeneratedComponent"


def _extract_review_notes(review_response: str) -> str:
    """Extrae las notas de revisión de la respuesta del LLM.

    Busca la línea que empieza con 'REVIEW_NOTES:'.

    Args:
        review_response: Respuesta cruda del LLM del paso de revisión.

    Returns:
        Notas de revisión como string, o cadena vacía si no se encuentran.
    """
    match = re.search(r"REVIEW_NOTES:\s*(.+?)(?:\n|$)", review_response, re.DOTALL)
    if match:
        return match.group(1).strip()
    return ""


def _extract_dependencies(templ_code: str) -> list[str]:
    """Extrae los atributos HTMX usados en el código generado.

    Args:
        templ_code: Código Go Templ completo.

    Returns:
        Lista de atributos hx-* únicos encontrados en el código.
    """
    hx_attrs = re.findall(r'(hx-[a-z-]+)=', templ_code)
    return sorted(set(hx_attrs))


def _pascal_to_snake(name: str) -> str:
    """Convierte PascalCase a snake_case para el nombre de archivo.

    Args:
        name: Nombre en PascalCase (ej: NodeCard).

    Returns:
        Nombre en snake_case (ej: node_card).
    """
    s1 = re.sub(r'(.)([A-Z][a-z]+)', r'\1_\2', name)
    return re.sub(r'([a-z0-9])([A-Z])', r'\1_\2', s1).lower()


# ─────────────────────────────────────────────────────────────────────────────
# CodeGenerator — Servicio Principal
# ─────────────────────────────────────────────────────────────────────────────

class CodeGenerator:
    """Genera código Go Templ usando LLM con ciclo generate → review obligatorio.

    Este servicio abstrae el proveedor LLM (Google Gemini / OpenAI) mediante
    LangChain. El cliente LLM debe pasarse en el constructor (inyección de
    dependencias) para garantizar el patrón singleton: un solo cliente por proceso.

    Attributes:
        llm: Instancia del cliente LLM de LangChain.
        settings: Configuración del servicio.
        _design_system_cache: Cache en memoria del contenido del design system.
    """

    def __init__(self, llm: BaseChatModel, settings: Settings) -> None:
        """Inicializa el generador con el cliente LLM y configuración.

        Args:
            llm: Cliente LLM de LangChain (ChatGoogleGenerativeAI o ChatOpenAI).
            settings: Objeto Settings con la configuración del servicio.
        """
        self.llm = llm
        self.settings = settings
        self._design_system_cache: str | None = None
        logger.info(
            "CodeGenerator inicializado | provider=%s model=%s",
            settings.llm_provider,
            settings.llm_model,
        )

    def _load_design_system(self) -> str:
        """Carga el design system reference desde disco (con cache en memoria).

        Returns:
            Contenido del archivo design_system_ref.md como string.

        Raises:
            FileNotFoundError: Si el archivo no existe en DESIGN_SYSTEM_REF_PATH.
        """
        if self._design_system_cache is not None:
            return self._design_system_cache

        path = Path(self.settings.design_system_ref_path)
        if not path.exists():
            raise FileNotFoundError(
                f"Design system reference no encontrado en: {path.resolve()}"
            )
        self._design_system_cache = path.read_text(encoding="utf-8")
        logger.info("Design system cargado desde %s (%d chars)", path, len(self._design_system_cache))
        return self._design_system_cache

    async def _invoke_llm(self, system: str, human: str) -> str:
        """Invoca el LLM con un par de mensajes (system + human).

        Usa tenacity para reintentar en caso de errores de red o de API.

        Args:
            system: Contenido del SystemMessage.
            human: Contenido del HumanMessage.

        Returns:
            Respuesta del LLM como string.

        Raises:
            RetryError: Si todos los intentos fallaron.
        """
        messages = [SystemMessage(content=system), HumanMessage(content=human)]
        try:
            async for attempt in AsyncRetrying(
                stop=stop_after_attempt(self.settings.max_retries),
                wait=wait_exponential(multiplier=1, min=2, max=10),
                reraise=True,
            ):
                with attempt:
                    response = await self.llm.ainvoke(messages)
                    return str(response.content)
        except RetryError as e:
            logger.error("LLM falló tras %d intentos: %s", self.settings.max_retries, e)
            raise

    async def _generate_raw(self, human_prompt: str) -> str:
        """Paso 1: Genera el código crudo (primer borrador).

        Args:
            human_prompt: Prompt formateado con la solicitud de generación.

        Returns:
            Código Go Templ extraído de la respuesta del LLM.
        """
        logger.debug("Iniciando paso 1: generación...")
        raw_response = await self._invoke_llm(SYSTEM_PROMPT, human_prompt)
        return _extract_templ_code(raw_response)

    async def _review_code(self, templ_code: str) -> tuple[str, str]:
        """Paso 2 (obligatorio): Revisa y corrige el código generado.

        Args:
            templ_code: Código Go Templ del primer borrador.

        Returns:
            Tupla (templ_code_revisado, review_notes).
        """
        logger.debug("Iniciando paso 2: revisión...")
        design_system = self._load_design_system()
        review_human = REVIEW_PROMPT.format(
            generated_code=templ_code,
            design_system_context=design_system,
        )
        review_response = await self._invoke_llm(SYSTEM_PROMPT, review_human)
        reviewed_code = _extract_templ_code(review_response)
        notes = _extract_review_notes(review_response)
        return reviewed_code, notes

    async def generate_component(
        self,
        description: str,
        existing_components: list[str],
    ) -> GenerationResult:
        """Genera un componente UI atómico o molecular en Go Templ.

        Ejecuta el ciclo obligatorio de dos pasos: generate → review.
        Aplica timeout total de GENERATION_TIMEOUT_SECONDS.

        Args:
            description: Descripción en lenguaje natural del componente a generar.
            existing_components: Lista de nombres de componentes Templ ya existentes
                                 en el proyecto (para evitar duplicados).

        Returns:
            GenerationResult con el código final revisado y metadatos.

        Raises:
            asyncio.TimeoutError: Si el ciclo supera GENERATION_TIMEOUT_SECONDS.
            RetryError: Si el LLM falla en todos los reintentos.
            FileNotFoundError: Si el design system reference no existe.
        """
        design_system = self._load_design_system()
        existing_str = "\n".join(f"- {c}" for c in existing_components) if existing_components else "_(ninguno aún)_"

        human_prompt = COMPONENT_GENERATION_PROMPT.format(
            component_description=description,
            design_system_context=design_system,
            existing_components=existing_str,
        )

        async def _full_cycle() -> GenerationResult:
            raw_code = await self._generate_raw(human_prompt)
            reviewed_code, notes = await self._review_code(raw_code)
            name = _extract_component_name(reviewed_code)
            filename = f"{_pascal_to_snake(name)}.templ"
            deps = _extract_dependencies(reviewed_code)
            logger.info(
                "Componente generado: %s → %s | deps=%s | review_notes=%s",
                name, filename, deps, notes[:80] if notes else "OK",
            )
            return GenerationResult(
                component_name=name,
                templ_code=reviewed_code,
                suggested_filename=filename,
                dependencies=deps,
                review_notes=notes,
            )

        return await asyncio.wait_for(
            _full_cycle(),
            timeout=self.settings.generation_timeout_seconds,
        )

    async def generate_page(
        self,
        description: str,
        sections: list[str],
        route_path: str,
    ) -> GenerationResult:
        """Genera una página completa (layout + secciones) en Go Templ.

        Ejecuta el mismo ciclo de dos pasos que generate_component.

        Args:
            description: Descripción en lenguaje natural de la página.
            sections: Lista de secciones a incluir en la página.
            route_path: Ruta HTTP a la que responde esta página (ej: "/nodes/{id}").

        Returns:
            GenerationResult con el código de página final revisado.

        Raises:
            asyncio.TimeoutError: Si el ciclo supera GENERATION_TIMEOUT_SECONDS.
            RetryError: Si el LLM falla en todos los reintentos.
        """
        design_system = self._load_design_system()
        sections_str = "\n".join(f"  {i+1}. {s}" for i, s in enumerate(sections))

        human_prompt = PAGE_GENERATION_PROMPT.format(
            page_description=description,
            design_system_context=design_system,
            existing_components="_(consultar el proyecto para componentes disponibles)_",
            page_sections=sections_str,
            route_path=route_path,
        )

        async def _full_cycle() -> GenerationResult:
            raw_code = await self._generate_raw(human_prompt)
            reviewed_code, notes = await self._review_code(raw_code)
            name = _extract_component_name(reviewed_code)
            filename = f"{_pascal_to_snake(name)}.templ"
            deps = _extract_dependencies(reviewed_code)
            logger.info(
                "Página generada: %s → %s | route=%s",
                name, filename, route_path,
            )
            return GenerationResult(
                component_name=name,
                templ_code=reviewed_code,
                suggested_filename=filename,
                dependencies=deps,
                review_notes=notes,
            )

        return await asyncio.wait_for(
            _full_cycle(),
            timeout=self.settings.generation_timeout_seconds,
        )


# ─────────────────────────────────────────────────────────────────────────────
# Factory — Construye el LLM client según el proveedor configurado
# ─────────────────────────────────────────────────────────────────────────────

def build_llm(settings: Settings) -> BaseChatModel:
    """Construye y devuelve el cliente LLM según LLM_PROVIDER.

    Este factory se llama UNA SOLA VEZ en main.py (patrón singleton).
    El resultado se inyecta en CodeGenerator.

    Args:
        settings: Objeto Settings con llm_provider, llm_api_key y llm_model.

    Returns:
        Instancia de BaseChatModel (ChatGoogleGenerativeAI o ChatOpenAI).

    Raises:
        ValueError: Si llm_provider no es "google" ni "openai".
        ImportError: Si el paquete langchain del proveedor no está instalado.
    """
    if settings.llm_provider == "google":
        from langchain_google_genai import ChatGoogleGenerativeAI
        logger.info("Inicializando LLM: Google Gemini (%s)", settings.llm_model)
        return ChatGoogleGenerativeAI(
            model=settings.llm_model,
            google_api_key=settings.llm_api_key,
            temperature=0.2,  # Baja temperatura: código consistente, no creativo
            convert_system_message_to_human=False,
        )
    elif settings.llm_provider == "openai":
        from langchain_openai import ChatOpenAI
        logger.info("Inicializando LLM: OpenAI (%s)", settings.llm_model)
        return ChatOpenAI(
            model=settings.llm_model,
            api_key=settings.llm_api_key,
            temperature=0.2,
        )
    else:
        raise ValueError(
            f"LLM_PROVIDER='{settings.llm_provider}' no es válido. "
            "Usa 'google' o 'openai'."
        )

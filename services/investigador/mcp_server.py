"""
mcp_server.py — Servidor FastMCP del Agente Investigador de Frontends.

Expone tres tools MCP:
  - generate_component: Genera un componente UI atómico en Go Templ.
  - generate_page: Genera una página completa en Go Templ.
  - list_design_tokens: Devuelve el design system de referencia completo.

El servidor se inicializa con create_server() que recibe el CodeGenerator
ya instanciado (singleton desde main.py).
"""
from __future__ import annotations

import asyncio
import logging
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from fastmcp import FastMCP

from config import Settings
from generator import CodeGenerator

logger = logging.getLogger(__name__)

# Instancia global del servidor MCP.
# Se configura con las tools mediante create_server().
mcp = FastMCP("nodal-investigador")

# Referencias globales inyectadas desde main.py antes de arrancar el servidor.
# Se usa un módulo-level dict para evitar globals mutables en closures.
_ctx: dict[str, Any] = {}


def create_server(generator: CodeGenerator, settings: Settings) -> FastMCP:
    """Configura el servidor FastMCP con las tools y devuelve la instancia.

    Este factory inyecta el CodeGenerator en el scope de las tools y registra
    todas las herramientas MCP disponibles.

    Args:
        generator: Instancia singleton de CodeGenerator ya inicializada.
        settings: Configuración del servicio (para leer DESIGN_SYSTEM_REF_PATH).

    Returns:
        Instancia FastMCP configurada y lista para arrancar.
    """
    _ctx["generator"] = generator
    _ctx["settings"] = settings
    logger.info(
        "Servidor FastMCP configurado | tools=[generate_component, generate_page, list_design_tokens]"
    )
    return mcp


def _log_invocation(tool_name: str, input_summary: str) -> None:
    """Registra en stdout la invocación de una tool MCP.

    Args:
        tool_name: Nombre de la tool invocada.
        input_summary: Resumen corto del input (primeros 120 chars).
    """
    ts = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    logger.info("[MCP] timestamp=%s tool=%s input=%s", ts, tool_name, input_summary[:120])


# ─────────────────────────────────────────────────────────────────────────────
# Tool 1: generate_component
# ─────────────────────────────────────────────────────────────────────────────

@mcp.tool(
    description=(
        "Genera un componente UI en Go Templ + HTMX + Tailwind CSS a partir de "
        "una descripción en lenguaje natural. El componente respeta el design system "
        "oscuro de Nodal y es revisado automáticamente antes de devolverse. "
        "Devuelve el código .templ listo para guardar en el proyecto."
    )
)
async def generate_component(
    description: str,
    existing_components: list[str] | None = None,
) -> dict[str, Any]:
    """Genera un componente UI atómico o molecular en Go Templ.

    Ejecuta el ciclo obligatorio generate → review antes de devolver el código.

    Args:
        description: Descripción en lenguaje natural del componente a generar.
                     Ejemplo: "Tarjeta de Nodo con título, descripción, badge de
                     categoría y botón para unirse con hx-post."
        existing_components: Lista opcional de nombres PascalCase de componentes
                             Templ ya existentes en el proyecto (para evitar
                             duplicados y facilitar composición).

    Returns:
        Dict con: component_name, templ_code, suggested_filename, dependencies,
        review_notes. En caso de error: {"error": str, "templ_code": None}.
    """
    _log_invocation("generate_component", description)
    generator: CodeGenerator = _ctx["generator"]
    components = existing_components or []

    try:
        result = await generator.generate_component(description, components)
        return {
            "component_name": result.component_name,
            "templ_code": result.templ_code,
            "suggested_filename": result.suggested_filename,
            "dependencies": result.dependencies,
            "review_notes": result.review_notes,
        }
    except asyncio.TimeoutError:
        logger.error("Timeout en generate_component | description=%s", description[:80])
        return {
            "error": "timeout: la generación superó el límite de tiempo configurado",
            "templ_code": None,
        }
    except Exception as e:
        logger.exception("Error en generate_component: %s", e)
        return {"error": str(e), "templ_code": None}


# ─────────────────────────────────────────────────────────────────────────────
# Tool 2: generate_page
# ─────────────────────────────────────────────────────────────────────────────

@mcp.tool(
    description=(
        "Genera una página completa (layout + secciones) en Go Templ para una "
        "ruta HTTP específica del proyecto Nodal. La página usa el componente "
        "Layout existente como wrapper y respeta el design system oscuro. "
        "Ideal para scaffolding rápido de nuevas rutas del backend Go."
    )
)
async def generate_page(
    description: str,
    sections: list[str],
    route_path: str,
) -> dict[str, Any]:
    """Genera una página completa en Go Templ.

    Args:
        description: Descripción en lenguaje natural de la página.
                     Ejemplo: "Página de detalle de un Nodo: muestra el título,
                     descripción, miembros activos y feed de mensajes recientes."
        sections: Lista de secciones a incluir.
                  Ejemplo: ["Hero con nombre del Nodo", "Lista de miembros",
                            "Feed de mensajes HTMX con scroll infinito",
                            "Formulario de unirse al Nodo"].
        route_path: Ruta HTTP de la página.
                    Ejemplo: "/nodes/{slug}" o "/admin/audit".

    Returns:
        Dict con: component_name, templ_code, suggested_filename, dependencies,
        review_notes. En caso de error: {"error": str, "templ_code": None}.
    """
    _log_invocation("generate_page", f"route={route_path} | {description[:60]}")
    generator: CodeGenerator = _ctx["generator"]

    try:
        result = await generator.generate_page(description, sections, route_path)
        return {
            "component_name": result.component_name,
            "templ_code": result.templ_code,
            "suggested_filename": result.suggested_filename,
            "dependencies": result.dependencies,
            "review_notes": result.review_notes,
        }
    except asyncio.TimeoutError:
        logger.error("Timeout en generate_page | route=%s", route_path)
        return {
            "error": "timeout: la generación de página superó el límite de tiempo",
            "templ_code": None,
        }
    except Exception as e:
        logger.exception("Error en generate_page: %s", e)
        return {"error": str(e), "templ_code": None}


# ─────────────────────────────────────────────────────────────────────────────
# Tool 3: list_design_tokens
# ─────────────────────────────────────────────────────────────────────────────

@mcp.tool(
    description=(
        "Devuelve el design system de referencia completo de Nodal en formato Markdown. "
        "Incluye paleta de colores, tipografía, componentes atómicos existentes, "
        "convenciones HTMX y reglas de accesibilidad. Útil para consultar qué "
        "componentes ya existen antes de solicitar la generación de uno nuevo."
    )
)
async def list_design_tokens() -> dict[str, str]:
    """Devuelve el contenido completo del design system reference.

    Lee el archivo design_system_ref.md y lo devuelve como string Markdown.
    Útil para que otros agentes o el backend consulten el design system
    antes de solicitar generación de componentes.

    Returns:
        Dict con clave "design_system" y el contenido Markdown como valor.
        En caso de error: {"error": str}.
    """
    _log_invocation("list_design_tokens", "")
    settings: Settings = _ctx["settings"]

    try:
        path = Path(settings.design_system_ref_path)
        content = path.read_text(encoding="utf-8")
        return {"design_system": content}
    except FileNotFoundError:
        return {
            "error": f"Design system reference no encontrado en: {settings.design_system_ref_path}"
        }
    except Exception as e:
        logger.exception("Error en list_design_tokens: %s", e)
        return {"error": str(e)}

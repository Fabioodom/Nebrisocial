"""
main.py — Punto de entrada del Agente Investigador de Frontends.

Responsabilidades:
  1. Validar configuración al arranque (LLM_API_KEY, proveedor, etc.)
  2. Inicializar el CodeGenerator como singleton (un solo cliente LLM)
  3. Inyectar el CodeGenerator en el servidor FastMCP
  4. Arrancar el servidor FastMCP en modo SSE (HTTP)
  5. Manejar señales SIGTERM/SIGINT para shutdown elegante

Ejecutar con:
  uv run python -m main          (desarrollo)
  python -m main                 (Docker, .venv en PATH)
"""
from __future__ import annotations

import asyncio
import logging
import signal
import sys

from config import Settings, settings as _settings
from generator import CodeGenerator, build_llm
from mcp_server import create_server

# ─────────────────────────────────────────────────────────────────────────────
# Logging
# ─────────────────────────────────────────────────────────────────────────────

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s — %(message)s",
    datefmt="%Y-%m-%dT%H:%M:%S",
    stream=sys.stdout,
)
logger = logging.getLogger("investigador.main")


# ─────────────────────────────────────────────────────────────────────────────
# Shutdown handler
# ─────────────────────────────────────────────────────────────────────────────

def _install_signal_handlers(loop: asyncio.AbstractEventLoop) -> None:
    """Instala handlers para SIGTERM y SIGINT (graceful shutdown).

    En Windows, signal.add_signal_handler no está disponible para SIGTERM,
    por lo que usamos signal.signal como fallback compatible.

    Args:
        loop: El event loop asyncio activo.
    """
    def _handle_exit(signum: int, frame: object) -> None:
        sig_name = signal.Signals(signum).name
        logger.info("Señal recibida: %s. Iniciando shutdown elegante...", sig_name)
        loop.stop()

    # Windows: signal.signal (no add_signal_handler para SIGTERM)
    signal.signal(signal.SIGINT, _handle_exit)
    try:
        signal.signal(signal.SIGTERM, _handle_exit)
    except (OSError, AttributeError):
        logger.warning("SIGTERM no disponible en esta plataforma (normal en Windows)")


# ─────────────────────────────────────────────────────────────────────────────
# Bootstrap
# ─────────────────────────────────────────────────────────────────────────────

async def main() -> None:
    """Función principal asíncrona del Agente Investigador.

    Ejecuta el ciclo de vida completo del microservicio:
    validación → inicialización singleton → arranque servidor MCP.
    """
    # 1. Validar configuración
    settings: Settings = _settings
    try:
        settings.validate()
    except ValueError as e:
        logger.critical("Error de configuración: %s", e)
        sys.exit(1)

    logger.info(
        "🔍 Iniciando Agente Investigador | provider=%s model=%s port=%d",
        settings.llm_provider,
        settings.llm_model,
        settings.mcp_port,
    )

    # 2. Construir el cliente LLM (singleton — solo se instancia una vez)
    try:
        llm_client = build_llm(settings)
    except Exception as e:
        logger.critical("Error al inicializar el cliente LLM: %s", e)
        sys.exit(1)

    # 3. Construir el CodeGenerator con el cliente LLM inyectado
    generator = CodeGenerator(llm=llm_client, settings=settings)

    # 4. Configurar el servidor FastMCP con el CodeGenerator
    server = create_server(generator=generator, settings=settings)

    # 5. Anunciar inicio
    print(
        f"\n🔍 Agente Investigador activo en "
        f"http://{settings.mcp_host}:{settings.mcp_port} "
        f"| LLM: {settings.llm_provider}/{settings.llm_model}\n",
        flush=True,
    )

    # 6. Arrancar el servidor FastMCP en modo SSE
    # FastMCP gestiona su propio loop uvicorn internamente.
    # run_async() arranca el servidor y bloquea hasta que se detiene.
    try:
        await server.run_async(
            transport="sse",
            host=settings.mcp_host,
            port=settings.mcp_port,
        )
    except Exception as e:
        logger.critical("Error fatal en el servidor FastMCP: %s", e)
        sys.exit(1)


if __name__ == "__main__":
    loop = asyncio.new_event_loop()
    asyncio.set_event_loop(loop)
    _install_signal_handlers(loop)
    try:
        loop.run_until_complete(main())
    except KeyboardInterrupt:
        logger.info("Agente Investigador detenido por el usuario.")
    finally:
        loop.close()
        logger.info("Event loop cerrado. Adiós. 👋")

# ─────────────────────────────────────────────────────────────────────────────
# Dockerfile multi-stage para el Agente Curador (Python 3.12 + uv)
# ─────────────────────────────────────────────────────────────────────────────
# Stage 1 (builder): instala dependencias con uv en un entorno virtual aislado.
# Stage 2 (runtime): imagen ligera que solo copia .venv y el código fuente.
# El binario `uv` NO aparece en la imagen final → imagen más pequeña y segura.
# ─────────────────────────────────────────────────────────────────────────────

# ── Stage 1: Builder ──────────────────────────────────────────────────────────
FROM python:3.12-slim AS builder

WORKDIR /app

# Bootstrap de uv (única excepción permitida al uso de pip, solo en build)
RUN pip install --no-cache-dir uv

# Copiar los archivos de definición de dependencias
COPY pyproject.toml uv.lock* ./

# Instalar dependencias en el entorno virtual .venv (sin el código del proyecto)
# --frozen: respeta exactamente el lock file
# --no-dev: excluye dependencias de desarrollo
# --no-install-project: no instala el paquete raíz todavía
RUN uv sync --frozen --no-dev --no-install-project

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM python:3.12-slim AS runtime

WORKDIR /app

# Copiar el entorno virtual completo del stage builder
COPY --from=builder /app/.venv /app/.venv

# Añadir .venv/bin al PATH para poder usar `python` y los scripts instalados
ENV PATH="/app/.venv/bin:$PATH"

# Copiar el código fuente del agente curador
COPY config.py mcp_server.py node_repo.py curator_agent.py main.py ./

# Variables de entorno con valores por defecto seguros
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    MCP_HOST=0.0.0.0 \
    MCP_PORT=8001 \
    EXTERNAL_API_TIMEOUT_SECONDS=10 \
    CACHE_TTL_SECONDS=86400

# Puerto del servidor FastMCP (SSE)
EXPOSE 8001

# Ejecutar directamente con python del .venv (sin uv run → imagen más ligera)
CMD ["python", "-m", "main"]

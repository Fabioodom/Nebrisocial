# 🕰️ Agente Cronista — Síntesis Diaria de Conocimiento

Microservicio Python que convierte conversaciones de chat efímeras en conocimiento persistente.

## ¿Qué hace?

Cada día a las **02:00 UTC**, el Cronista:
1. Detecta los Nodos con actividad de chat superior al umbral configurado.
2. Filtra el ruido de los mensajes (mensajes cortos, URLs, emojis, duplicados).
3. Divide los mensajes en chunks y los sintetiza con **LangChain Map-Reduce**.
4. Publica el resumen como un hilo `[Resumen IA]` en el Nodo, con `is_ai_generated=TRUE`.
5. Registra todo en `agent_audit_log`.

## Stack

- **Python 3.12** + **uv** (gestor de paquetes)
- **APScheduler** — Cron job asíncrono
- **LangChain** — Map-Reduce sobre chunks de mensajes
- **psycopg2** — Acceso a PostgreSQL
- **tiktoken** — Conteo de tokens real (cl100k_base)
- **FastMCP** — Tool MCP para lectura de chats

## Inicialización (desarrollo)

```bash
# Desde la raíz del monorepo:
cd services/cronista

# Crear entorno virtual e instalar dependencias
uv sync

# Copiar y editar variables de entorno
cp .env.example .env
# Editar .env y añadir LLM_API_KEY

# Ejecutar síntesis manualmente (una vez)
uv run python -m main --run-now

# Ejecutar como servicio (scheduler + MCP server)
uv run python -m main
```

## Docker

```bash
# Construir solo el cronista
docker compose build cronista

# Arrancar todos los servicios
docker compose up -d

# Ver logs
docker compose logs -f cronista
```

## Variables de Entorno

| Variable | Por defecto | Descripción |
|---|---|---|
| `DATABASE_URL` | `postgresql://...@localhost:5432/nodal_db` | DSN de PostgreSQL |
| `LLM_PROVIDER` | `openai` | `openai` o `google` |
| `LLM_API_KEY` | _(vacío)_ | Clave API del proveedor LLM |
| `LLM_MODEL` | `gpt-4o-mini` | Nombre del modelo |
| `MIN_MESSAGES_THRESHOLD` | `20` | Mínimo de mensajes para generar resumen |
| `MAX_SUMMARY_WORDS` | `800` | Máximo de palabras en el resumen final |
| `CHUNK_SIZE_TOKENS` | `500` | Tokens máximos por chunk |
| `CRON_HOUR` | `2` | Hora UTC del cron job |
| `CRON_MINUTE` | `0` | Minuto UTC del cron job |
| `MCP_HOST` | `0.0.0.0` | Host del servidor FastMCP |
| `MCP_PORT` | `8002` | Puerto del servidor FastMCP |

## Tools MCP disponibles

- **`get_node_recent_chats(node_id, hours=24)`** — Lee los últimos mensajes de un nodo.
- **`list_active_nodes(min_messages=20)`** — Lista nodos elegibles para resumen.

## Reglas PRD

- **R1**: Solo procesa nodos sin resumen IA en las últimas 24h.
- **R2**: El resumen no supera 800 palabras.
- **R3**: Nunca se exponen usernames reales. Se usa "Un participante".
- **R4**: Los hilos publicados son `is_ai_generated=TRUE`, `pinned=TRUE`.
- **R5**: Si un nodo falla, se registra en `agent_audit_log` y se continúa.

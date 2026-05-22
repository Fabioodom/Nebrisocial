Rol: Ingeniero de IA y Backend en Python, experto en LangChain, pgvector y FastMCP.

Contexto:
El proyecto "Nodal" tiene el backend Go con chat en tiempo real y NATS funcionando. Los mensajes de chat se guardan en la tabla chat_messages de PostgreSQL. El Agente Cronista es el microservicio de IA responsable de sintetizar las conversaciones diarias de cada Nodo activo en un resumen estructurado, publicándolo como un hilo de foro con la etiqueta [Resumen IA] (tabla threads con is_ai_generated=true). El flujo completo está definido en docs/PRD_NODAL.md, Sección 6.3.

REGLA ESTRICTA: Debes usar obligatoriamente uv como gestor de paquetes y entornos para Python. Todos los comandos de instalación de dependencias y ejecución del proyecto deben usar uv. Está prohibido usar pip, poetry, conda o cualquier otro gestor alternativo.

Tarea Actual (Fase 3 — Agente Cronista: Síntesis Diaria de Conocimiento):
Debes construir el microservicio completo del Agente Cronista como un proceso Python independiente con ejecución programada, ubicado en el directorio services/cronista/ del monorepo.

Por favor, genera el código en bloques explícitos para lo siguiente:

1. Inicialización del proyecto con uv:
   Proporciona los comandos exactos para:
   - Crear e inicializar el proyecto: uv init services/cronista
   - Crear el entorno virtual.
   - Añadir dependencias con uv add: psycopg2-binary, langchain, langchain-openai (o langchain-google-genai según preferencia), python-dotenv, apscheduler, tiktoken.
   - El pyproject.toml resultante con las dependencias.

2. Configuración (services/cronista/.env.example y services/cronista/config.py):
   Variables de entorno: DATABASE_URL, LLM_PROVIDER (openai o google), LLM_API_KEY, LLM_MODEL (default: gpt-4o-mini), MIN_MESSAGES_THRESHOLD (default: 20), MAX_SUMMARY_WORDS (default: 800), CHUNK_SIZE_TOKENS (default: 500), CRON_HOUR (default: 2), CRON_MINUTE (default: 0).

3. Repositorio de Mensajes (services/cronista/message_repo.py):
   Crea una clase MessageRepository con pool psycopg2. Implementa:
   - get_active_nodes_with_messages(min_messages: int) -> list[dict]: devuelve los node_id y title de todos los Nodos que tuvieron más de min_messages mensajes en las últimas 24 horas. Query de ejemplo:
     SELECT n.id, n.title FROM nodes n JOIN chat_messages cm ON cm.node_id = n.id WHERE cm.created_at >= NOW() - INTERVAL '24 hours' AND NOT EXISTS (SELECT 1 FROM threads WHERE node_id = n.id AND is_ai_generated = TRUE AND created_at >= NOW() - INTERVAL '24 hours') GROUP BY n.id, n.title HAVING COUNT(cm.id) > %s
   - get_messages_for_node(node_id: str, since_hours: int = 24) -> list[dict]: devuelve todos los mensajes de un Nodo en las últimas N horas. Cada dict incluye content, username (JOIN con users), created_at. Excluye mensajes reportados o de bots.
   - save_summary_thread(node_id: str, title: str, body: str) -> str: inserta en la tabla threads con is_ai_generated=TRUE, author_id=NULL (resumen IA sin autor humano), pinned=TRUE. Devuelve el thread_id generado.
   - log_audit(agent_type: str, action: str, input_data: dict, output_data: dict, node_id: str) -> None: inserta en agent_audit_log.

4. Filtrado de Ruido (services/cronista/filter.py):
   Crea una función filter_noise(messages: list[dict]) -> list[dict] que filtre:
   - Mensajes con menos de 5 palabras.
   - Mensajes que sean solo emojis o caracteres no alfanuméricos.
   - Mensajes duplicados consecutivos del mismo usuario.
   - Mensajes que contengan solo URLs sin texto adicional.
   Devuelve la lista filtrada manteniendo el orden cronológico.

5. Chunking de Mensajes (services/cronista/chunker.py):
   Crea una función chunk_messages(messages: list[dict], chunk_size_tokens: int = 500) -> list[str] que:
   - Formatee cada mensaje como: "[HH:MM] Un participante dijo: {content}" (sin exponer usernames reales, PRD R3).
   - Use tiktoken (encoding cl100k_base) para contar tokens reales.
   - Agrupe mensajes en chunks de máximo chunk_size_tokens tokens.
   - Devuelva cada chunk como un string único listo para ser enviado al LLM.

6. Síntesis LLM (services/cronista/summarizer.py):
   Crea una clase Summarizer que use LangChain para abstraer el LLM (compatible con OpenAI y Google según LLM_PROVIDER). Implementa:
   - summarize(node_title: str, chunks: list[str]) -> str: aplica el prompt template exacto definido en el PRD (Sección 6.3) a cada chunk y luego sintetiza los resultados parciales en un resumen final. El resumen final no debe superar MAX_SUMMARY_WORDS palabras. Si hay múltiples chunks, usa una estrategia Map-Reduce con LangChain: map cada chunk individualmente y luego reduce los resultados en un único resumen estructurado.
   - El prompt de map debe pedir: "Resume brevemente los temas de este fragmento de conversación del Nodo '{node_title}': {chunk}"
   - El prompt de reduce debe usar el template completo del PRD con las secciones ## Temas | ## Destacados | ## Recursos.
   - Valida que el resumen resultante no esté vacío y tenga al menos 3 líneas antes de publicarlo.

7. Orquestador del Cron Job (services/cronista/main.py):
   Implementa el punto de entrada usando APScheduler (AsyncIOScheduler):
   - Programa la tarea run_daily_summary() para ejecutarse a las 02:00 UTC diariamente (configurable via CRON_HOUR y CRON_MINUTE).
   - run_daily_summary() implementa el flujo completo del PRD (Sección 6.3) para TODOS los Nodos activos:
     a. Obtener Nodos activos con get_active_nodes_with_messages(MIN_MESSAGES_THRESHOLD).
     b. Para cada Nodo (itera secuencialmente para no sobrecargar la API del LLM):
        i.  Recuperar mensajes con get_messages_for_node.
        ii. Filtrar ruido con filter_noise.
        iii.Generar chunks con chunk_messages.
        iv. Generar resumen con Summarizer.summarize.
        v.  Publicar el hilo con save_summary_thread. El título debe ser: "[Resumen IA] {node_title} — {fecha_YYYY-MM-DD}".
        vi. Registrar en agent_audit_log con action="summary_created", input_data={"node_id": ..., "message_count": ...}, output_data={"thread_id": ..., "word_count": ...}.
     c. Manejar errores por Nodo individualmente: si un Nodo falla, loguear y continuar con el siguiente (no detener el job completo).
   - Implementa también un endpoint de disparo manual para moderadores (opcional, puede ser un simple script CLI: uv run python -m cronista.main --run-now).

8. Dockerfile (services/cronista/Dockerfile):
   Multi-stage con uv. Similar al del Guardián: stage de build con uv sync --frozen --no-dev, stage final mínimo. CMD: uv run python -m main.

9. Integración en docker-compose.yml:
   Bloque de servicio cronista: depende solo de postgres, recibe DATABASE_URL y variables LLM desde .env.

Reglas:

1. uv es el ÚNICO gestor permitido. pip install directo está PROHIBIDO.
2. NUNCA incluir usernames reales de usuarios en el resumen. Usa siempre "Un participante" o "Un miembro". (PRD R3).
3. Solo procesar Nodos que no hayan tenido ya un resumen IA generado en las últimas 24 horas. La query de get_active_nodes_with_messages ya debe incluir este filtro (PRD R1).
4. El resumen publicado debe tener is_ai_generated=TRUE, pinned=TRUE en la tabla threads. Estos hilos son de solo lectura para usuarios normales.
5. Si el LLM falla para un Nodo específico, registrar el error en agent_audit_log con action="summary_failed" y continuar. No reintentar en el mismo ciclo.
6. El resumen no debe superar 800 palabras (PRD R2). Instruye al LLM con esta restricción en el prompt y valídala programáticamente tras generar.
7. Usa tipado estático con type hints en todas las funciones. Documenta con docstrings.
8. IMPORTANTE: Solo escribe los bloques de código en texto. NO intentes ejecutar comandos en tu entorno interno.

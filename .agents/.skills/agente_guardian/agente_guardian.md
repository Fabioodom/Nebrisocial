Rol: Ingeniero de IA y Backend en Python, experto en LangChain, pgvector y FastMCP.

Contexto:
El proyecto "Nodal" tiene el backend Go funcionando con NATS integrado. Cuando un usuario intenta crear un Nodo, el backend Go emite el evento node.creation.requested en NATS con el payload: {"node_id": "...", "title": "...", "description": "...", "owner_id": "...", "requested_at": "..."}. El Agente Guardián debe suscribirse a ese evento, decidir si el Nodo es semánticamente duplicado y responder al backend con su decisión. Todo el proceso queda registrado en la tabla agent_audit_log de PostgreSQL compartida. El esquema completo de la BD está en docs/PRD_NODAL.md, Sección 5.

REGLA ESTRICTA: Debes usar obligatoriamente uv como gestor de paquetes y entornos para Python. Todos los comandos de instalación de dependencias y ejecución del proyecto deben usar uv. Está prohibido usar pip, poetry, conda o cualquier otro gestor alternativo.

Tarea Actual (Fase 3 — Agente Guardián: Clasificador Semántico de Duplicados):
Debes construir el microservicio completo del Agente Guardián como un proceso Python independiente, ubicado en el directorio services/guardian/ del monorepo.

Por favor, genera el código en bloques explícitos para lo siguiente:

1. Inicialización del proyecto con uv:
   Proporciona los comandos exactos para:
   - Crear e inicializar el proyecto: uv init services/guardian
   - Crear el entorno virtual y activarlo.
   - Añadir las dependencias necesarias con uv add: nats-py, sentence-transformers, psycopg2-binary, python-dotenv, langchain-community (opcional para utilidades).
   - El archivo pyproject.toml resultante con las dependencias listadas.

2. Configuración (services/guardian/.env.example y services/guardian/config.py):
   Variables de entorno necesarias: NATS_URL, DATABASE_URL (formato DSN PostgreSQL), SIMILARITY_THRESHOLD_BLOCK (default: 0.95), SIMILARITY_THRESHOLD_SUGGEST (default: 0.85), EMBEDDING_MODEL (default: all-MiniLM-L6-v2), MAX_SIMILAR_CANDIDATES (default: 5).
   Crea un módulo config.py que lea estas variables con python-dotenv y exponga un objeto Settings con tipos validados.

3. Generación de Embeddings (services/guardian/embeddings.py):
   Crea una clase EmbeddingService que:
   - Inicialice el modelo sentence-transformers/all-MiniLM-L6-v2 en el constructor. El modelo debe cargarse una sola vez (singleton) para no re-inicializarlo en cada request. Confirma: dimensión 384, latencia <50ms en CPU.
   - Implemente generate(text: str) -> list[float]: recibe el texto (título + " " + descripción concatenados) y devuelve el vector de 384 dimensiones como lista de floats.

4. Repositorio pgvector (services/guardian/vector_repo.py):
   Crea una clase VectorRepository con un pool de conexiones psycopg2. Implementa:
   - search_similar_nodes(embedding: list[float], threshold: float, limit: int) -> list[dict]: ejecuta la consulta pgvector de similitud coseno sobre la tabla node_embeddings. La query debe ser:
     SELECT ne.node_id, n.title, n.slug, 1 - (ne.embedding <=> %s::vector) AS similarity FROM node_embeddings ne JOIN nodes n ON ne.node_id = n.id WHERE 1 - (ne.embedding <=> %s::vector) > %s ORDER BY similarity DESC LIMIT %s
     Devuelve lista de dicts con node_id, title, slug, similarity.
   - save_embedding(node_id: str, embedding: list[float]) -> None: inserta o actualiza el embedding de un Nodo en node_embeddings (INSERT ... ON CONFLICT (node_id) DO UPDATE SET embedding = EXCLUDED.embedding).
   - log_audit(agent_type: str, action: str, input_data: dict, output_data: dict, confidence: float, node_id: str) -> None: inserta un registro en agent_audit_log.

5. Lógica de Decisión (services/guardian/decision.py):
   Crea la función evaluate_similarity(similar_nodes: list[dict], config: Settings, timeout: bool = False) -> dict que aplique los umbrales del PRD (Sección 6.2):
   - Si timeout=True y el candidato más similar (si lo hay) tiene similarity <= SIMILARITY_THRESHOLD_BLOCK: retornar {"decision": "approve", "needs_review": True} (PRD R3). Si timeout=True pero hay similitud >THRESHOLD_BLOCK, BLOQUEAR igualmente (PRD R1 es estricto).
   - Si el candidato más similar tiene similarity > SIMILARITY_THRESHOLD_BLOCK: retorna {"decision": "block", "similar_node": {node_id, title, slug, similarity}, "reason": "Nodo semánticamente idéntico ya existe."}.
   - Si similarity > SIMILARITY_THRESHOLD_SUGGEST: retorna {"decision": "suggest", "candidates": [lista hasta MAX_SIMILAR_CANDIDATES], "reason": "Existen Nodos similares. Considera unirte a uno de ellos."}.
   - Si no hay candidatos o similarity < SIMILARITY_THRESHOLD_SUGGEST: retorna {"decision": "approve"}.
   Esta función debe ser pura: sin efectos secundarios, sin acceso a BD, solo lógica de comparación. Facilita el testing unitario.

6. Suscriptor NATS y Lógica Principal (services/guardian/main.py):
   Implementa el punto de entrada async con asyncio:
   - Conecta a NATS usando nats-py (librería asyncio-compatible). Configura reconexión automática.
   - Suscribe al subject node.creation.requested usando nc.subscribe con callback asíncrono.
   - El callback guardian_handler(msg) debe implementar el flujo completo del PRD (Sección 6.2):
     a. Deserializar el payload JSON del mensaje NATS.
     b. Concatenar event["title"] + " " + event["description"].
     c. Generar el embedding con EmbeddingService.generate().
     d. Consultar pgvector con VectorRepository.search_similar_nodes usando SIMILARITY_THRESHOLD_SUGGEST como umbral mínimo y un timeout asyncio de 2 segundos (PRD R3). Si el timeout expira, pasar timeout=True a evaluate_similarity.
     e. Evaluar la decisión con evaluate_similarity.
     f. Registrar la decisión en agent_audit_log con VectorRepository.log_audit (agent_type="guardian", action=decision["decision"], confidence=max_similarity_encontrada o 0.0).
     g. Si el mensaje tiene reply subject (NATS Request-Reply), responder con: await nc.publish(msg.reply, json.dumps(decision).encode()).
   - Implementar manejo de errores con try/except en el callback: si cualquier paso falla (excepto el bloqueo por similitud), responder con {"decision": "approve", "needs_review": True, "error": str(e)} para no bloquear al usuario.
   - El proceso debe correr indefinidamente con un asyncio event loop, manejando señales SIGTERM/SIGINT para un shutdown elegante que cierre la conexión NATS y el pool de psycopg2.

7. Dockerfile (services/guardian/Dockerfile):
   Crea un Dockerfile multi-stage que use uv para instalar dependencias:
   - Stage 1 (builder): FROM python:3.12-slim. Instala uv con pip install uv (solo esta vez, para bootstrap del build). Copia pyproject.toml y uv.lock. Ejecuta uv sync --frozen --no-dev --no-install-project.
   - Stage 2 (runtime): FROM python:3.12-slim. Copia el entorno virtual .venv del stage 1 y el código fuente. Añade .venv/bin al PATH. CMD: ["python", "-m", "main"] (sin uv run en producción para imagen más ligera).

8. Integración en docker-compose.yml:
   Muéstrame el bloque de servicio guardian que debo añadir al docker-compose.yml del proyecto. Debe: construir desde services/guardian/Dockerfile, depender de postgres y nats (con condition: service_healthy si es posible), recibir las variables de entorno desde un archivo .env, y tener restart: unless-stopped.

Reglas:

1. uv es el ÚNICO gestor de paquetes y entornos permitido. Cualquier instrucción que use pip install directamente en el flujo de desarrollo está PROHIBIDA (solo se permite en el Dockerfile para el bootstrap de uv en el stage de build).
2. El modelo all-MiniLM-L6-v2 debe cargarse una sola vez al arrancar el servicio (patrón singleton instanciado en main.py antes de entrar al event loop). No en cada mensaje recibido.
3. La conexión a PostgreSQL debe usar un pool de conexiones (psycopg2.pool.ThreadedConnectionPool). No crear una nueva conexión por mensaje.
4. La lógica de decisión (evaluate_similarity) debe estar en un módulo separado y ser pura (sin efectos secundarios). Esto es obligatorio para mantener el código testeable.
5. El umbral de bloqueo (>0.95) es ESTRICTO según PRD R1: nunca aprobar un Nodo con similitud mayor a ese valor. La única excepción es el timeout de pgvector (PRD R3), y aun así, si hay un hit con similitud >0.95 ya conocido en la respuesta parcial, se BLOQUEA igualmente.
6. Todo error y toda decisión deben registrarse en agent_audit_log antes de responder al backend, incluso los errores de degradación elegante.
7. Usa tipado estático con type hints en todas las funciones. Documenta cada función con docstring indicando parámetros, retorno y posibles excepciones.
8. IMPORTANTE: Solo escribe los bloques de código en texto. NO intentes ejecutar comandos en tu entorno interno.

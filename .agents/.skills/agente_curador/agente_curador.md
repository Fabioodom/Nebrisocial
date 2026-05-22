Rol: Ingeniero de IA y Backend en Python, experto en LangChain, pgvector y FastMCP.

Contexto:
El proyecto "Nodal" tiene el Agente Guardián funcionando. Cuando el Guardián aprueba la creación de un Nodo, el backend Go emite el evento node.created en NATS con el payload: {"node_id": "...", "slug": "...", "title": "...", "description": "...", "category": "...", "created_at": "..."}. El Agente Curador debe suscribirse a ese evento, determinar qué APIs externas son relevantes según la categoría del Nodo, invocar las herramientas (tools) correspondientes para obtener metadatos enriquecidos, y actualizar el campo metadata (JSONB) de la tabla nodes en PostgreSQL. El servidor MCP expone estas herramientas de forma estandarizada. El diseño completo está en docs/PRD_NODAL.md, Sección 6.4.

REGLA ESTRICTA: Debes usar obligatoriamente uv como gestor de paquetes y entornos para Python. Todos los comandos de instalación de dependencias y ejecución del proyecto deben usar uv. Está prohibido usar pip, poetry, conda o cualquier otro gestor alternativo.

Tarea Actual (Fase 3 — Agente Curador + Servidor FastMCP):
Debes construir el microservicio completo del Agente Curador, que incluye tanto el servidor FastMCP (que expone las tools de APIs externas) como el agente suscriptor de NATS que orquesta la llamada a esas tools. Todo en el directorio services/curator/ del monorepo.

Por favor, genera el código en bloques explícitos para lo siguiente:

1. Inicialización del proyecto con uv:
   Proporciona los comandos exactos para:
   - uv init services/curator
   - uv add: fastmcp, httpx, nats-py, psycopg2-binary, python-dotenv, cachetools, tenacity.
   - El pyproject.toml resultante.

2. Configuración (services/curator/.env.example y services/curator/config.py):
   Variables de entorno: NATS_URL, DATABASE_URL, MCP_HOST (default: 0.0.0.0), MCP_PORT (default: 8001), EXTERNAL_API_TIMEOUT_SECONDS (default: 10), CACHE_TTL_SECONDS (default: 86400), RAWG_API_KEY (opcional, para la API de RAWG).
   El objeto Settings debe exponer un mapa CATEGORY_TO_TOOLS: dict[str, list[str]] que mapee categorías a nombres de tools. Ejemplo: {"manga": ["manga_metadata"], "anime": ["manga_metadata"], "videojuegos": ["game_metadata"], "cine": ["movie_metadata"], "musica": ["music_metadata"], "libros": ["book_metadata"], "pokemon": ["pokemon_metadata"], "tecnologia": ["tech_metadata"]}.

3. Servidor FastMCP con las Tools (services/curator/mcp_server.py):
   Crea el servidor MCP usando FastMCP("nodal-curator"). Implementa las siguientes tools asíncronas con httpx.AsyncClient. Cada tool debe:
   - Tener su decorador @mcp.tool() con descripción.
   - Implementar un timeout de EXTERNAL_API_TIMEOUT_SECONDS.
   - Usar cachetools.TTLCache para cachear respuestas por 24h (PRD R5). La clave de caché es el argumento de búsqueda.
   - Usar tenacity para reintentar hasta 2 veces en caso de error de red (wait_fixed=1s).
   - Devolver un dict con los datos obtenidos o lanzar una excepción descriptiva si la API falla.

   Tools a implementar:
   a) manga_metadata(title: str) -> dict: consulta GET https://api.jikan.moe/v4/manga?q={title}&limit=1. Devuelve el dict del PRD (Sección 6.4): title_jp, chapters, author, synopsis (máx 300 chars), cover_url, mal_id, genres.
   b) game_metadata(title: str) -> dict: consulta GET https://api.rawg.io/api/games?search={title}&key={RAWG_API_KEY}&page_size=1. Devuelve: name, released, rating, platforms (lista de nombres), genres, background_image, rawg_id.
   c) movie_metadata(title: str) -> dict: consulta GET https://api.themoviedb.org/3/search/movie?query={title}&api_key={TMDB_API_KEY}. Devuelve: title, release_date, overview (máx 300 chars), poster_path (URL completa), vote_average, genres.
   d) music_metadata(artist_or_album: str) -> dict: consulta GET https://musicbrainz.org/ws/2/release/?query={artist_or_album}&fmt=json&limit=1. Devuelve: title, artist_credit, date, genres.
   e) book_metadata(title: str) -> dict: consulta GET https://openlibrary.org/search.json?title={title}&limit=1. Devuelve: title, author_name, first_publish_year, number_of_pages_median, cover_url.
   f) pokemon_metadata(name: str) -> dict: consulta GET https://pokeapi.co/api/v2/pokemon/{name.lower()}. Devuelve: name, id, types (lista), height, weight, sprite_url (front_default), abilities (lista).
   g) tech_metadata(repo_name: str) -> dict: consulta GET https://api.github.com/search/repositories?q={repo_name}&sort=stars&per_page=1. Devuelve: full_name, description, stargazers_count, language, html_url, topics.

4. Repositorio de Nodos (services/curator/node_repo.py):
   Implementa con psycopg2:
   - get_node(node_id: str) -> dict: recupera title, description, category, metadata (JSONB) de la tabla nodes.
   - update_node_metadata(node_id: str, new_metadata: dict) -> None: ejecuta UPDATE nodes SET metadata = metadata || %s::jsonb WHERE id = %s. Usa el operador de concatenación de JSONB (||) para no sobrescribir campos existentes (PRD R2).
   - log_audit(agent_type: str, action: str, input_data: dict, output_data: dict, node_id: str) -> None: inserta en agent_audit_log.

5. Lógica del Agente Curador (services/curator/curator_agent.py):
   Crea una clase CuratorAgent que reciba el Settings, el NodeRepository y la instancia del cliente MCP. Implementa:
   - async def curate(self, node_id: str, title: str, category: str) -> None: flujo principal del PRD (Sección 6.4):
     a. Determinar las tools relevantes consultando CATEGORY_TO_TOOLS[category.lower()]. Si la categoría no está mapeada, registrar "no_tools_available" en audit y salir.
     b. Para cada tool relevante, invocarla pasando title como argumento principal.
     c. Agregar los resultados de todas las tools invocadas en un único dict enriched_metadata.
     d. Verificar que el Nodo no tiene metadatos manuales ya establecidos (PRD R2): hacer get_node y si metadata ya contiene claves distintas de las que va a escribir, no sobrescribir esas claves.
     e. Actualizar nodes.metadata con update_node_metadata.
     f. Registrar en agent_audit_log con agent_type="curator", action="metadata_enriched", input_data={"node_id": ..., "category": ..., "tools_used": [...]}, output_data={"enriched_fields": list(enriched_metadata.keys())}.
   - Manejar el timeout (PRD R3): si una tool excede EXTERNAL_API_TIMEOUT_SECONDS, loguear y continuar con la siguiente sin bloquear.

6. Suscriptor NATS y Main (services/curator/main.py):
   Implementa el punto de entrada async:
   - Inicia el servidor FastMCP en modo SSE o stdio en un task asyncio paralelo.
   - Conecta a NATS y suscribe al subject node.created.
   - El callback curator_handler(msg) deserializa el payload y llama a CuratorAgent.curate(node_id, title, category) dentro de un try/except. Si falla, el Nodo se crea sin metadatos (PRD R4, degradación elegante).
   - El servidor FastMCP y el loop de NATS deben correr concurrentemente usando asyncio.gather.

7. Dockerfile (services/curator/Dockerfile):
   Multi-stage con uv: stage de build con uv sync --frozen --no-dev, stage final mínimo. Expone el puerto MCP_PORT. CMD: uv run python -m main.

8. Integración en docker-compose.yml:
   Bloque de servicio curator: depende de postgres y nats, expone el puerto 8001, recibe variables de entorno desde .env.

Reglas:

1. uv es el ÚNICO gestor permitido. pip install directo está PROHIBIDO.
2. NUNCA sobrescribir campos de metadata que el usuario haya editado manualmente (PRD R2). El operador JSONB || de PostgreSQL y la verificación previa en curator_agent.py garantizan esto.
3. Timeout estricto de EXTERNAL_API_TIMEOUT_SECONDS por llamada a API externa (PRD R3). Usa httpx timeout= en el cliente.
4. Si una API externa falla después de los reintentos de tenacity, la tool lanza una excepción, el Agente la captura, la registra y continúa con las demás tools. El Nodo NUNCA queda bloqueado por un fallo de API externa (PRD R4).
5. El caché de respuestas de APIs externas se mantiene en memoria (TTLCache). En producción se podría migrar a Redis, pero para el MVP la caché en proceso es suficiente.
6. La categoría del Nodo se normaliza a minúsculas antes de buscar en CATEGORY_TO_TOOLS para evitar errores por capitalización.
7. Usa tipado estático con type hints en todas las funciones. Documenta con docstrings.
8. IMPORTANTE: Solo escribe los bloques de código en texto. NO intentes ejecutar comandos en tu entorno interno.

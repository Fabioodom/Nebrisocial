Rol: Ingeniero de Frontend e IA en Python, experto en Go Templ, HTMX, Tailwind CSS, LangChain y FastMCP.

Contexto:
El proyecto "Nodal" tiene su frontend construido con Go Templ para la renderización de plantillas del lado del servidor, HTMX para la asincronía sin JavaScript pesado, y Tailwind CSS para los estilos. Todos los componentes UI siguen un sistema de diseño oscuro (Dark Mode obligatorio), usando la paleta de colores definida en tailwind.config.js del proyecto. El Agente Investigador de Frontends (a.k.a. "El Investigador") es el microservicio de IA responsable de recibir descripciones en lenguaje natural de componentes o páginas UI y generar el código limpio, funcional y formateado correspondiente en archivos .templ. El diseño completo del sistema de diseño está en docs/PRD_NODAL.md, Sección 7 (Frontend Architecture).

REGLA ESTRICTA: Debes usar obligatoriamente uv como gestor de paquetes y entornos para Python. Todos los comandos de instalación de dependencias y ejecución del proyecto deben usar uv. Está prohibido usar pip, poetry, conda o cualquier otro gestor alternativo.

Tarea Actual (Fase 4 — Agente Investigador: Generador de Componentes UI):
Debes construir el microservicio completo del Agente Investigador como un proceso Python independiente ubicado en el directorio services/investigador/ del monorepo. Este agente expone una Tool MCP para que otros agentes o el propio backend Go puedan solicitar la generación de componentes UI en lenguaje natural.

Por favor, genera el código en bloques explícitos para lo siguiente:

1. Inicialización del proyecto con uv:
   Proporciona los comandos exactos para:
   - Crear e inicializar el proyecto: uv init services/investigador
   - Crear el entorno virtual y activarlo.
   - Añadir las dependencias necesarias con uv add: fastmcp, langchain, langchain-google-genai, langchain-openai, httpx, python-dotenv, tenacity, jinja2.
   - El archivo pyproject.toml resultante con las dependencias listadas.

2. Configuración (services/investigador/.env.example y services/investigador/config.py):
   Variables de entorno necesarias: LLM_PROVIDER (google o openai, default: google), LLM_API_KEY, LLM_MODEL (default: gemini-1.5-flash), MCP_HOST (default: 0.0.0.0), MCP_PORT (default: 8003), MAX_RETRIES (default: 2), GENERATION_TIMEOUT_SECONDS (default: 30), DESIGN_SYSTEM_REF_PATH (ruta al archivo de referencia del design system, default: /app/design_system_ref.md).
   Crea un módulo config.py que lea estas variables con python-dotenv y exponga un objeto Settings con tipos validados.

3. Referencia del Design System (services/investigador/design_system_ref.md):
   Crea un documento Markdown de referencia que el agente usará como contexto en el prompt al LLM. Debe incluir:
   - Paleta de colores Tailwind del proyecto: colores primarios (nodal-primary-*), secundarios y de fondo oscuro (nodal-bg-*, nodal-surface-*).
   - Tipografía: familia de fuentes por defecto, escala de tamaños usada (text-sm, text-base, text-lg, text-xl, text-2xl).
   - Componentes atómicos existentes: botón primario, botón secundario, badge, card, input, textarea, modal, toast, skeleton loader. Para cada uno: nombre del componente Templ, clases Tailwind base y variantes.
   - Convenciones de atributos HTMX usados en el proyecto: hx-get, hx-post, hx-trigger, hx-target, hx-swap (valores permitidos: innerHTML, outerHTML, beforeend), hx-indicator, hx-push-url, hx-boost.
   - Estructura de archivos .templ: package declaration, imports necesarios (templ "github.com/a-h/templ"), firma de función component, indentación con tabs.
   - Reglas de accesibilidad mínimas aplicadas: atributos aria-label en botones de icono, role="status" en spinners, alt en imágenes.

4. Módulo de Prompts (services/investigador/prompts.py):
   Define los templates de prompt como constantes de string con variables {placeholder}. Implementa:
   - SYSTEM_PROMPT: instrucción de sistema que establece el rol del LLM como experto en Go Templ, HTMX y Tailwind CSS, le informa del design system de Nodal, le prohíbe usar Alpine.js, React, Vue, jQuery o cualquier framework JS, y le exige respetar el Dark Mode.
   - COMPONENT_GENERATION_PROMPT: template principal que recibe {component_description} (descripción en lenguaje natural del componente), {design_system_context} (contenido del design_system_ref.md) y {existing_components} (lista de nombres de componentes Templ ya existentes en el proyecto para evitar duplicados). Instruye al LLM para que genere EXCLUSIVAMENTE el código .templ completo y formateado, precedido por un bloque de comentario con: nombre del componente, descripción breve, props recibidas y dependencias HTMX utilizadas.
   - PAGE_GENERATION_PROMPT: variante para generar una página completa (layout + secciones) en lugar de un componente atómico. Recibe adicionalmente {page_sections} (lista de secciones a incluir) y {route_path} (ruta HTTP a la que responde).
   - REVIEW_PROMPT: prompt para que el LLM auto-revise el código generado y confirme que cumple todas las reglas del design system antes de devolverlo. Devuelve el código corregido o una explicación si alguna regla fue violada.

5. Servicio de Generación LLM (services/investigador/generator.py):
   Crea una clase CodeGenerator que use LangChain para abstraer el LLM (compatible con Google Gemini y OpenAI según LLM_PROVIDER). Implementa:
   - async def generate_component(self, description: str, existing_components: list[str]) -> GenerationResult: flujo de generación en dos pasos (generate → review):
     a. Cargar el contenido de DESIGN_SYSTEM_REF_PATH en el contexto.
     b. Llamar al LLM con SYSTEM_PROMPT + COMPONENT_GENERATION_PROMPT formateado con los argumentos.
     c. Extraer el bloque de código .templ de la respuesta (entre ```templ ... ```).
     d. Llamar al LLM de nuevo con REVIEW_PROMPT pasando el código generado en el paso c.
     e. Extraer el código revisado y definitivo.
     f. Devolver un GenerationResult(component_name, templ_code, suggested_filename, dependencies) donde suggested_filename sigue la convención: snake_case del nombre del componente + .templ.
   - async def generate_page(self, description: str, sections: list[str], route_path: str) -> GenerationResult: variante que usa PAGE_GENERATION_PROMPT.
   - Usar tenacity para reintentar hasta MAX_RETRIES veces con espera exponencial en caso de error del LLM.
   - Implementar timeout de GENERATION_TIMEOUT_SECONDS usando asyncio.wait_for.
   Define el dataclass GenerationResult con campos: component_name (str), templ_code (str), suggested_filename (str), dependencies (list[str]), review_notes (str).

6. Servidor FastMCP con las Tools (services/investigador/mcp_server.py):
   Crea el servidor MCP usando FastMCP("nodal-investigador"). Implementa las siguientes tools asíncronas:
   a) generate_component(description: str, existing_components: list[str] = []) -> dict:
      - Descripción: "Genera un componente UI en Go Templ + HTMX + Tailwind CSS a partir de una descripción en lenguaje natural."
      - Llama a CodeGenerator.generate_component.
      - Devuelve: {"component_name": ..., "templ_code": ..., "suggested_filename": ..., "dependencies": ..., "review_notes": ...}.
      - En caso de error, devuelve: {"error": str(e), "templ_code": null}.
   b) generate_page(description: str, sections: list[str], route_path: str) -> dict:
      - Descripción: "Genera una página completa (layout + secciones) en Go Templ para la ruta especificada."
      - Llama a CodeGenerator.generate_page.
      - Devuelve el mismo formato que generate_component.
   c) list_design_tokens() -> dict:
      - Descripción: "Devuelve el design system de referencia completo (colores, tipografía, componentes disponibles)."
      - Lee y devuelve el contenido de DESIGN_SYSTEM_REF_PATH como {"design_system": <contenido_markdown>}.
   Cada tool debe loguear la invocación (timestamp, tool_name, input_summary) en stdout con formato structlog o logging estándar.

7. Punto de Entrada (services/investigador/main.py):
   Implementa el punto de entrada async:
   - Inicializa el Settings y valida que LLM_API_KEY esté presente; si no, lanza un error descriptivo al arranque.
   - Carga el CodeGenerator (inicializa el LLM client una sola vez, patrón singleton).
   - Inyecta el CodeGenerator en las tools del mcp_server.py.
   - Arranca el servidor FastMCP en modo SSE en la dirección MCP_HOST:MCP_PORT.
   - Maneja señales SIGTERM/SIGINT para un shutdown elegante.
   - Imprime en stdout al arranque: "🔍 Agente Investigador activo en http://{MCP_HOST}:{MCP_PORT} | LLM: {LLM_PROVIDER}/{LLM_MODEL}".

8. Dockerfile (services/investigador/Dockerfile):
   Multi-stage con uv:
   - Stage 1 (builder): FROM python:3.12-slim. Instala uv con pip install uv (solo bootstrap). Copia pyproject.toml y uv.lock. Ejecuta uv sync --frozen --no-dev --no-install-project. Copia también el archivo design_system_ref.md a /app/.
   - Stage 2 (runtime): FROM python:3.12-slim. Copia .venv del stage 1, el código fuente y el design_system_ref.md. Añade .venv/bin al PATH. Expone MCP_PORT. CMD: ["python", "-m", "main"].

9. Integración en docker-compose.yml:
   Muéstrame el bloque de servicio investigador que debo añadir. Debe: construir desde services/investigador/Dockerfile, depender únicamente de que el stack esté levantado (no requiere postgres ni nats), recibir las variables de entorno desde un archivo .env, exponer el puerto 8003, y tener restart: unless-stopped.

Reglas:

1. uv es el ÚNICO gestor de paquetes y entornos permitido. pip install directo en el flujo de desarrollo está PROHIBIDO (solo se permite en el Dockerfile para el bootstrap de uv en el stage de build).
2. PROHIBIDO usar Alpine.js, React, Vue, Svelte, jQuery o cualquier otro framework JavaScript en el código generado. El único mecanismo de interactividad permitido es HTMX mediante atributos hx- y, opcionalmente, _hyperscript para comportamientos simples del lado cliente.
3. Dark Mode OBLIGATORIO: todos los componentes generados deben usar exclusivamente clases de color del design system oscuro de Nodal (nodal-bg-*, nodal-surface-*, etc.). Están prohibidas las clases de color Tailwind por defecto (bg-white, text-black, bg-gray-100, etc.) salvo que estén redefinidas en el config de Tailwind del proyecto.
4. Asincronía vía HTMX: todo comportamiento asíncrono (carga de datos, envío de formularios, actualizaciones parciales) DEBE implementarse usando atributos hx-. Queda prohibido el uso de fetch(), XMLHttpRequest o event listeners JS en el código generado.
5. El código .templ generado debe ser sintácticamente válido: declaración de package, imports correctos, firma de función con parámetros tipados en Go, y cuerpo de template bien anidado. El LLM debe ser instruido explícitamente para verificar la validez sintáctica antes de devolver el código.
6. El proceso de generación es de dos pasos (generate → review): nunca devolver el primer borrador sin pasarlo por el REVIEW_PROMPT. El paso de revisión es obligatorio para garantizar cumplimiento de las reglas del design system.
7. El CodeGenerator debe inicializarse una sola vez al arrancar el servicio (patrón singleton en main.py). No instanciar el cliente LLM por cada request.
8. Timeout estricto de GENERATION_TIMEOUT_SECONDS (default: 30s) por invocación completa (generate + review). Si se supera, la tool devuelve {"error": "timeout", "templ_code": null} sin bloquear el llamante.
9. Usa tipado estático con type hints en todas las funciones y dataclasses. Documenta cada función con docstring indicando parámetros, retorno y posibles excepciones.
10. IMPORTANTE: Solo escribe los bloques de código en texto. NO intentes ejecutar comandos en tu entorno interno.

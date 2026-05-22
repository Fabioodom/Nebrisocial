Rol: Desarrollador Go Senior experto en Arquitecturas Limpias, PostgreSQL, Templ y htmx.

Contexto:
El proyecto "Nodal" ya tiene autenticación JWT, chat en tiempo real con WebSockets y NATS integrado. Las tablas threads y thread_replies ya existen en PostgreSQL (según el esquema de docs/PRD_NODAL.md, Sección 5.1). Es el momento de implementar el foro de discusión de cada Nodo: la funcionalidad que permite a los usuarios crear hilos de debate y responder a ellos, siendo esta la vía principal para preservar conocimiento según el PRD (Sección 6.3).

Tarea Actual (Fase 2 — CRUD de Hilos de Foro con Templ y htmx):
Debes implementar el sistema completo de hilos de foro. La interacción del usuario debe ser fluida usando htmx: las acciones de crear hilo, ver hilo, añadir respuesta y listar hilos no deben recargar la página completa.

Por favor, genera el código en bloques explícitos para lo siguiente:

1. Repositorio de Hilos (internal/platform/database/thread_repo.go):
   Crea los structs Thread y ThreadReply (mapeados a sus tablas SQL). Implementa las siguientes funciones con database/sql:
   - CreateThread(db *sql.DB, nodeID, authorID, title, body string) (*Thread, error)
   - GetThreadsByNode(db *sql.DB, nodeID string, page, pageSize int) ([]Thread, error): lista paginada de hilos de un Nodo, ordenados por pinned DESC, created_at DESC. Excluye el campo body en el listado (solo título, autor, fecha y conteo de respuestas).
   - GetThreadByID(db *sql.DB, threadID string) (*Thread, error): recupera el hilo completo con su body.
   - DeleteThread(db *sql.DB, threadID, requestingUserID string, requestingUserRole string) error: solo el autor o un moderador/owner pueden borrar. Implementa la lógica de autorización aquí.
   - PinThread(db *sql.DB, threadID, nodeID, requestingUserRole string) error: solo moderators y owners pueden fijar hilos.
   Crea el archivo internal/platform/database/reply_repo.go con:
   - CreateReply(db *sql.DB, threadID, authorID, body string) (*ThreadReply, error)
   - GetRepliesByThread(db *sql.DB, threadID string) ([]ThreadReply, error)
   - DeleteReply(db *sql.DB, replyID, requestingUserID string) error

2. Handlers HTTP del Foro (internal/handlers/forum.go):
   Implementa los siguientes handlers. Los que muten estado deben requerir autenticación via el middleware RequireAuth:
   - GET /nodes/{nodeID}/threads → lista de hilos (paginada, soporta query param ?page=N)
   - POST /nodes/{nodeID}/threads → crea un hilo; responde con el fragmento HTML del nuevo hilo para que htmx lo prependa a la lista (hx-swap="afterbegin")
   - GET /nodes/{nodeID}/threads/{threadID} → vista detalle del hilo con sus respuestas
   - DELETE /nodes/{nodeID}/threads/{threadID} → elimina un hilo; responde con HTML vacío para que htmx elimine el elemento del DOM
   - PUT /nodes/{nodeID}/threads/{threadID}/pin → fija/desfija un hilo (solo moderator/owner); responde con el fragmento actualizado
   - POST /nodes/{nodeID}/threads/{threadID}/replies → añade una respuesta; responde con el fragmento HTML de la nueva respuesta para que htmx lo añada al final de la lista
   - DELETE /nodes/{nodeID}/threads/{threadID}/replies/{replyID} → elimina una respuesta

3. Vistas Templ (internal/handlers/views/forum/):
   Crea los siguientes componentes Templ:
   - thread_list.templ: componente ThreadList(threads []Thread, nodeID string, currentPage int) que renderice la lista de hilos con paginación. Cada hilo muestra: título, autor, fecha, número de respuestas, badge "📌 Fijado" si está pinned, badge "[Resumen IA]" si is_ai_generated es true. Incluye un botón "Nuevo Hilo" que abra un formulario inline con htmx (hx-get sobre un partial).
   - thread_form.templ: componente ThreadForm(nodeID string) con inputs para título y body (textarea). El formulario usa hx-post="/nodes/{nodeID}/threads" y hx-target="#thread-list" con hx-swap="afterbegin". Incluye validación básica de longitud (título: máx 200 chars, body: máx 10000 chars) con atributos HTML.
   - thread_detail.templ: componente ThreadDetail(thread Thread, replies []ThreadReply, nodeID string, currentUserID string, currentUserRole string) que renderice el cuerpo del hilo y la lista de respuestas. Incluye el formulario de nueva respuesta al final (hx-post sobre el endpoint de replies, hx-target="#reply-list", hx-swap="beforeend"). Los botones de borrar/fijar solo se muestran si el currentUserID es el autor o el rol es moderator/owner.
   - reply_item.templ: componente ReplyItem(reply ThreadReply, currentUserID string) para un item individual de respuesta, con botón de eliminar (hx-delete, hx-confirm para confirmación, hx-target="closest .reply-item", hx-swap="outerHTML swap:1s").

4. Validación de Input (internal/handlers/forum.go):
   Antes de cualquier escritura en BD, valida:
   - Título del hilo: entre 3 y 200 caracteres.
   - Body del hilo: entre 10 y 10.000 caracteres.
   - Body de la respuesta: entre 1 y 5.000 caracteres.
   Si la validación falla, devuelve un fragmento HTML de error (un <div class="error">) en lugar de un código de error HTTP puro, para que htmx pueda mostrarlo en el target correspondiente.

5. Actualización de Rutas (cmd/nodal/main.go):
   Muéstrame solo el bloque de líneas exactas de mux.HandleFunc que debo añadir para todas las rutas del foro, indicando cuáles deben estar envueltas con el middleware RequireAuth y cuáles con RequireRole("moderator", "owner").

Reglas:

1. Usa database/sql sin ORM. Toda la lógica SQL debe usar prepared statements o queries parametrizadas para prevenir SQL injection.
2. La autorización de acciones sensibles (borrar, fijar) se verifica tanto en el repositorio (para operaciones de BD) como en el handler (para devolver 403 antes de tocar la BD).
3. Los hilos con is_ai_generated = true son de solo lectura para los usuarios: no se pueden editar ni borrar manualmente.
4. Todas las respuestas htmx deben devolver el fragmento HTML mínimo necesario, nunca la página completa. El Content-Type de estas respuestas debe ser text/html.
5. La paginación del listado de hilos devuelve como máximo 20 hilos por página.
6. Manejo de errores estricto. Logs estructurados en JSON.
7. IMPORTANTE: Solo escribe los bloques de código en texto. NO intentes ejecutar comandos en tu entorno interno.

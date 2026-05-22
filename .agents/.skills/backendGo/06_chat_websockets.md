Rol: Desarrollador Go Senior experto en Arquitecturas Limpias, PostgreSQL, Templ y htmx.

Contexto:
El proyecto "Nodal" ya cuenta con autenticación JWT funcionando (middleware RequireAuth disponible), la tabla chat_messages creada en PostgreSQL con su índice idx_chat_node_time (según el esquema de docs/PRD_NODAL.md, Sección 5.1), y el servidor HTTP nativo en Go corriendo. Aún no hay integración con NATS (eso vendrá en el prompt siguiente). El objetivo de este paso es implementar el chat en tiempo real dentro de la vista de un Nodo usando WebSockets nativos de Go.

Tarea Actual (Fase 2 — Chat en Tiempo Real con WebSockets):
Debes construir la infraestructura completa de WebSockets en Go para el chat dentro de un Nodo. Cada Nodo tendrá su propio "Hub" de conexiones activas. Los mensajes se guardan en PostgreSQL y se retransmiten a todos los clientes del mismo Nodo.

Por favor, genera el código en bloques explícitos para lo siguiente:

1. Repositorio de Chat (internal/platform/database/chat_repo.go):
   Crea el struct ChatMessage (mapea a la tabla chat_messages). Implementa:
   - SaveMessage(db *sql.DB, nodeID, userID, content string) (*ChatMessage, error): inserta el mensaje en la BD.
   - GetRecentMessages(db *sql.DB, nodeID string, limit int) ([]ChatMessage, error): recupera los últimos N mensajes del Nodo ordenados por created_at ASC. Usa el índice idx_chat_node_time.

2. Hub de WebSockets por Nodo (internal/chat/hub.go):
   Implementa una estructura Hub con las siguientes características:
   - Mantiene un map de clientes conectados (clients map[*Client]bool).
   - Tiene canales para: register (nuevas conexiones), unregister (desconexiones) y broadcast (mensajes a difundir a todos los clientes del hub).
   - El método Run() del Hub debe ejecutarse en una goroutine y gestionar el ciclo de vida de los tres canales en un único select loop.
   - Debe existir un HubManager (internal/chat/hub_manager.go): un singleton con mutex que gestiona un map de nodeID → *Hub, creando el Hub si no existe y devolviéndolo si ya está activo. Esto permite que cada Nodo tenga su propio espacio de chat aislado.

3. Cliente WebSocket (internal/chat/client.go):
   Implementa la estructura Client con los campos: conn (*websocket.Conn), hub (*Hub), send (chan []byte) y userID, username (string).
   - readPump(): goroutine que lee mensajes del WebSocket. Al recibir un mensaje, lo guarda en PostgreSQL (llamando a SaveMessage) y lo envía al canal broadcast del Hub. Gestiona el cierre de conexión con pong handlers y deadlines.
   - writePump(): goroutine que escucha el canal send y escribe mensajes al WebSocket. Implementa ping periódico (cada 30s) para detectar conexiones muertas.
   Usa la librería github.com/gorilla/websocket para el upgrade y la gestión de la conexión.

4. Handler HTTP del Chat (internal/handlers/chat.go):
   Crea el handler ServeWS(hubManager *HubManager, db *sql.DB, w http.ResponseWriter, r *http.Request) que:
   - Aplique el upgrade HTTP → WebSocket con un Upgrader configurado con CheckOrigin apropiado.
   - Extraiga el nodeID de los path params (e.g., /nodes/{nodeID}/ws).
   - Valide que el usuario autenticado (obtenido del contexto via el middleware RequireAuth) sea miembro del Nodo antes de permitir la conexión.
   - Obtenga o cree el Hub del Nodo via HubManager.
   - Registre el nuevo Client en el Hub.
   - Lance las goroutines readPump y writePump del cliente.
   - Antes de lanzar las goroutines, envíe al cliente los últimos 100 mensajes (GetRecentMessages) serializados como JSON para poblar el historial inicial.

5. Vista Templ del Chat (internal/handlers/views/chat.templ):
   Crea un componente ChatView(nodeID, nodeName string, initialMessages []ChatMessage) que renderice:
   - Un contenedor #chat-messages con los mensajes del historial inicial.
   - Un input de texto y botón de envío.
   - El script JavaScript mínimo e inline necesario para: abrir el WebSocket (ws://... o wss://... según protocolo), recibir mensajes y añadirlos al DOM (#chat-messages), y enviar mensajes al presionar el botón o Enter. No uses frameworks JS externos.

6. Actualización de Rutas (cmd/nodal/main.go):
   Muéstrame solo las líneas exactas que debo añadir para registrar:
   - GET /nodes/{nodeID}/ws → handler ServeWS (protegido con RequireAuth).
   - GET /nodes/{nodeID}/chat → handler que renderice la vista ChatView (protegido con RequireAuth).

Reglas:

1. Usa github.com/gorilla/websocket para la gestión de WebSockets. No uses librerías de terceros para el Hub (impleméntalo de cero con goroutines y channels nativos de Go).
2. readPump y writePump deben ejecutarse siempre en goroutines separadas por cliente.
3. El Hub nunca debe bloquear en el broadcast: si el canal send de un cliente está lleno, cierra esa conexión y la da de baja del Hub (cliente lento = desconectado).
4. Todos los mensajes enviados por WebSocket deben estar serializados en JSON con la estructura: {"user_id": "...", "username": "...", "content": "...", "created_at": "..."}.
5. El campo content de un mensaje no puede superar los 2000 caracteres. Valida esto en readPump antes de guardar en BD.
6. Los mensajes de chat en historial deben cargarse de BD de forma paginada (límite 100 por defecto).
7. Usa database/sql sin ORM. Manejo de errores estricto.
8. IMPORTANTE: Solo escribe los bloques de código en texto. NO intentes ejecutar comandos en tu entorno interno.

Rol: Desarrollador Go Senior experto en Arquitecturas Limpias, PostgreSQL, Templ y htmx.

Contexto:
El proyecto "Nodal" ya tiene el chat en tiempo real funcionando con WebSockets nativos de Go (Hub por Nodo con goroutines). El problema actual es que si escalamos a múltiples instancias del backend Go, los mensajes de chat solo se propagan dentro de la instancia que los recibe, no a las demás. Además, necesitamos un canal asíncrono para comunicar el backend con los agentes de IA. La solución definida en el PRD (Sección 3.2 y 4.3) es integrar NATS como bus de mensajes inter-instancia.

Tarea Actual (Fase 2 — Integración NATS para Escalabilidad y Eventos):
Debes integrar NATS en el backend Go para dos propósitos concretos:
  A) Propagar mensajes de chat entre instancias horizontales del backend (fanout por Nodo).
  B) Emitir el evento asíncrono node.creation.requested cuando un usuario intente crear un Nodo, para que el Agente Guardián lo procese.

Por favor, genera el código en bloques explícitos para lo siguiente:

1. Conexión NATS (internal/platform/messaging/nats.go):
   Crea una función ConnectNATS() (*nats.Conn, error) que lea la URL del servidor NATS de la variable de entorno NATS_URL (default: nats://localhost:4222). Configura opciones robustas: reconexión automática (MaxReconnects: -1, ReconnectWait: 2s), y un manejador de error de reconexión que loguee el evento en JSON. Usa la librería github.com/nats-io/nats.go.

2. Actualización del docker-compose.yml:
   Añade el servicio NATS al archivo docker-compose.yml existente. La imagen debe ser nats:latest. Expone el puerto 4222 (clientes) y 8222 (HTTP monitoring). Configura el servicio para que arranque junto a PostgreSQL.

3. Propagación de Chat entre Instancias (internal/chat/nats_relay.go):
   Crea una estructura NATSRelay que conecte el Hub de cada Nodo con NATS. Implementa:
   - PublishChatMessage(nc *nats.Conn, nodeID string, payload []byte) error: publica un mensaje en el subject nodal.chat.{nodeID}.
   - SubscribeChatMessages(nc *nats.Conn, nodeID string, hub *Hub) (*nats.Subscription, error): suscribe el Hub al subject nodal.chat.{nodeID}. Al recibir un mensaje de NATS, lo envía al canal broadcast del Hub para que se propague a todos los clientes WebSocket de esa instancia.
   Modifica el readPump del Client (internal/chat/client.go) para que, en lugar de enviar directamente al canal broadcast del Hub local, publique el mensaje en NATS via PublishChatMessage. Así, el relay se encarga de distribuirlo a todas las instancias.

4. Evento de Creación de Nodo (internal/platform/messaging/events.go):
   Define las estructuras de los eventos como tipos Go con tags JSON:
   - NodeCreationRequestedEvent: campos node_id (string), title (string), description (string), owner_id (string), requested_at (time.Time).
   - NodeCreatedEvent: campos node_id (string), slug (string), created_at (time.Time).
   Implementa las funciones:
   - PublishNodeCreationRequested(nc *nats.Conn, event NodeCreationRequestedEvent) error: serializa el struct a JSON y publica en el subject node.creation.requested.
   - PublishNodeCreated(nc *nats.Conn, event NodeCreatedEvent) error: publica en node.created.

5. Integración en el Handler de Nodos (internal/handlers/node.go):
   Modifica el handler de creación de Nodo (POST /nodes) para que:
   a) Antes de insertar en la BD, emita el evento node.creation.requested via PublishNodeCreationRequested y espere la respuesta del Agente Guardián. Para el MVP, implementa un mecanismo de Request-Reply usando nats.Conn.RequestWithContext con un timeout de 3 segundos. Si el agente responde con {"decision": "block"}, devuelve un error 409 Conflict con el mensaje del agente. Si responde {"decision": "suggest", "similar_node_id": "..."}, devuelve un 200 con la sugerencia. Si responde {"decision": "approve"} o el timeout expira (degradación elegante), procede con la creación.
   b) Tras insertar el Nodo exitosamente en la BD, emita el evento node.created via PublishNodeCreated.

6. Inyección de Dependencias en main (cmd/nodal/main.go):
   Muéstrame el bloque de código completo que debo añadir en la función main() para: inicializar la conexión NATS, manejar el error de conexión con log.Fatal si NATS no está disponible, y pasar la conexión *nats.Conn a los handlers y al NATSRelay como dependencia (sin variables globales, usa inyección explícita por parámetro o struct).

Reglas:

1. Usa github.com/nats-io/nats.go. No uses JetStream en este prompt (puede venir en iteraciones futuras).
2. El flujo de chat DEBE pasar siempre por NATS como intermediario, incluso si solo hay una instancia. Esto garantiza que la arquitectura sea horizontalmente escalable desde el inicio.
3. Degradación elegante obligatoria: si NATS está caído, el handler de creación de Nodo debe proceder con la creación directamente (loguear el fallo de NATS pero no bloquear al usuario). El chat puede degradarse a solo-local-instance en ese caso.
4. Todos los subjects NATS deben seguir la convención de nomenclatura: nodal.{entidad}.{acción} (e.g., nodal.chat.{nodeID}, node.creation.requested, node.created).
5. No uses variables globales para la conexión NATS. Inyéctala explícitamente.
6. Manejo de errores estricto. Logs estructurados en JSON para todos los eventos NATS relevantes (conexión, desconexión, error de publish, recepción de mensaje).
7. IMPORTANTE: Solo escribe los bloques de código en texto. NO intentes ejecutar comandos en tu entorno interno.

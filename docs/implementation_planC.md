# Nodal Chat UI Redesign and WebSockets Integration Plan

This plan details the steps to redesign the chat experience inside Nodal communities (`node_detail.templ`) and activate real-time features using Gorilla WebSockets and HTMX.

## User Review Required

> [!IMPORTANT]
> To prevent duplicate messages on the sender's client (receiving the message once via the HTTP response and again via the WebSocket broadcast), the HTTP POST handler `PostChatMessageHandler` will return an empty response (`204 No Content`). The client will render the message only when it arrives through the WebSocket broadcast. This is the standard, most robust pattern for real-time HTMX applications.

## Proposed Changes

### Dependencies
We will add the Gorilla WebSocket library as the server's WebSocket engine.

- Run `go get github.com/gorilla/websocket`

---

### WebSocket Infrastructure

#### [NEW] [hub.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/platform/websocket/hub.go)
- Create a thread-safe `Hub` that manages client connections grouped by `NodeID`.
- Create a `Client` struct that wraps a `websocket.Conn` and manages its write/read loops (`writePump`/`readPump`) with ping/pong keepalives.
- Expose a `Broadcast(nodeID string, message []byte)` method to send messages to all clients in a specific community.

---

### Handlers & Routing

#### [MODIFY] [node.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/node.go)
- Update `PostChatMessageHandler` signature to receive the `*websocket.Hub`.
- In `PostChatMessageHandler`:
  - After inserting the chat message in the DB, render the new `ChatMessageBroadcast` template component into a buffer.
  - Broadcast this HTML slice to the hub under the node's ID.
  - Return `204 No Content` to the poster client.
- Add `WebSocketHandler(hub *websocket.Hub) http.HandlerFunc`:
  - Upgrade the HTTP connection to WebSocket using Gorilla Upgrader.
  - Create a new `Client` registered to the correct `nodeID`.
  - Start the read and write pumps in goroutines.

#### [MODIFY] [main.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/cmd/nodal/main.go)
- Initialize the `websocket.Hub` and start it in a background goroutine: `go hub.Run()`.
- Pass the `hub` instance to `PostChatMessageHandler`.
- Register the routing check for `/nodes/{id}/ws` in the main mux path handler.

---

### UI & HTMX Templates

#### [MODIFY] [node_detail.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/node_detail.templ)
- Import `"strings"` for category badge lowercase mapping.
- Load HTMX WebSocket extension script: `<script src="https://unpkg.com/htmx.org/dist/ext/ws.js"></script>`.
- Replace the wrapper container with `.node-detail-container` to set a viewport-fit flex grid context.
- Update `.node-detail-header` to highlighted clean typography and a category badge, removing heavy cards.
- Add `hx-ext="ws" ws-connect="/nodes/{id}/ws"` to the `.chat-section`.
- Redesign message template by renaming `ChatMessageFragment` to `ChatMessageItem`. Layout details should resemble a Discord message (avatar + body with header author/timestamp and text content).
- Define `ChatMessageBroadcast` component:
  ```templ
  templ ChatMessageBroadcast(m *database.ChatMessage) {
      <div id="chat-messages" hx-swap-oob="beforeend">
          @ChatMessageItem(m)
      </div>
  }
  ```
- Update chat form `hx-swap` to `"none"` since UI updates are fully handled by WebSocket broadcasts.
- Add a MutationObserver script to scroll `#chat-messages` automatically when a new message is appended.

---

### Styling Layer

#### [MODIFY] [components.css](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/static/css/components.css)
- Define `.node-detail-container` with `height: calc(100vh - 57px)` and `overflow: hidden`.
- Customize `.node-detail-header` with bottom border and clean padding.
- Estilize `.chat-timeline` with `flex: 1` and `overflow-y: auto`.
- Add Discord-style styling for `.chat-message`, `.chat-avatar`, `.chat-message-body`, `.chat-message-header`, `.chat-message-author`, `.chat-message-time`, and `.chat-message-text`.
- Customize `.chat-input-area` to be sticky, with `bg-bg-surface` rounded inputs (`--radius-pill`), and compact sizing (`max-height: 80px`).

---

## Verification Plan

### Automated Tests
- Build templates: `go run github.com/a-h/templ/cmd/templ generate`
- Run compile/build test: `go build ./cmd/nodal`
- Run Go unit tests: `go test ./...`

### Manual Verification
- Launch server: `go run ./cmd/nodal/main.go`.
- Open two browser tabs on `/nodes/{id}`.
- Send a message in tab 1. Verify that the message instantly appends on both tab 1 and tab 2.
- Verify the chat input area is sticky and rounded.
- Verify that the chat auto-scrolls to the bottom on receiving messages.

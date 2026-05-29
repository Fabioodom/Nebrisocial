# Implementation Plan: Backend Integration (WebSocket & Node Chat Event Flow)

This plan details the steps required to verify, refine, and connect the WebSocket Realtime Hub and Node Chat event flow in the backend, aligning with the presentation layer.

## User Review Required

> [!NOTE]
> The modern routing syntax (`METHOD /path/{param}`) is supported in the current Go version (`go 1.26.3`). Changing the catch-all `/nodes/` handler to explicit routes makes route handling cleaner, more secure, and eliminates custom route-parsing conditionals.

## Proposed Changes

### Routing & Initialization (main.go)

#### [MODIFY] [main.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/cmd/nodal/main.go)
- Confirm that the `websocket.NewHub()` is initialized and run (`go hub.Run()`) before starting the HTTP listener. (Already verified: present at lines 35–36).
- Replace the legacy `/nodes/` catch-all mux mapping with explicit, typed Go 1.22+ routes:
  - `GET /nodes/{id}/ws` mapped to `handlers.WebSocketHandler(hub)`
  - `POST /nodes/{id}/chat` mapped to `handlers.PostChatMessageHandler(db, nc, hub)`
  - `GET /nodes/{id}` mapped to `handlers.NodeDetailHandler(db)` (to preserve detail page rendering under specific routing)

---

### Controller & Handlers (node.go)

#### [MODIFY] [node.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/node.go)
- Verify `PostChatMessageHandler` sequence exactly follows the required order:
  1. Insert chat message into database via `database.CreateChatMessage`.
  2. Publish the NATS event `chat.message.sent` for the Chronicler agent.
  3. Render the HTML fragment `views.ChatMessageBroadcast(msg)` using a byte buffer.
  4. Broadcast the HTML fragment via `hub.Broadcast(nodeID, fragmentHTML)` to all connected WS clients of the node.
  5. Return `204 No Content` HTTP response code to the POST sender.
  *(This is already correct but we will ensure it compiles clean.)*

---

### Frontend Attributes (node_detail.templ)

#### [MODIFY] [node_detail.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/node_detail.templ)
- Verify `.chat-section` container has the HTMX WS extension attributes: `hx-ext="ws"` and `ws-connect={ "/nodes/" + node.ID + "/ws" }`.
  *(Verified: already present and correct on line 62. No changes required.)*

## Verification Plan

### Automated Tests
- Execute `go build ./...` from the repository root to verify complete compilation.
- Ensure no type mismatches or routing conflicts.

### Manual Verification
- Once compiled, the app can be run locally (`go run ./cmd/nodal/main.go`), and WebSocket connections can be checked by navigating to a node page and sending chat messages.

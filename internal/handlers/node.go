package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	gorilla "github.com/gorilla/websocket"
	nats "github.com/nats-io/nats.go"

	"nodal/internal/auth"
	"nodal/internal/handlers/views"
	"nodal/internal/platform/database"
	"nodal/internal/platform/websocket"
)

// nodeCreationPayload es el payload que se envía al Agente Guardián
// vía NATS Request-Reply (subject: node.creation.requested).
type nodeCreationPayload struct {
	NodeID      string `json:"node_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"`
	OwnerID     string `json:"owner_id"`
	RequestedAt string `json:"requested_at"`
}

// guardianDecision es la respuesta esperada del Agente Guardián.
type guardianDecision struct {
	Decision    string `json:"decision"`     // "approve" | "block" | "suggest"
	Reason      string `json:"reason"`       // presente si decision != "approve"
	NeedsReview bool   `json:"needs_review"` // true si el guardián tuvo un error de degradación
}

// nodeCreatedEvent es el payload del evento de éxito que se publica
// al Agente Curador (subject: node.created).
type nodeCreatedEvent struct {
	NodeID      string `json:"node_id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"`
	CreatedAt   string `json:"created_at"`
}

// chatMessageSentEvent es el payload que se publica al Agente Cronista
// cada vez que se inserta un mensaje en un nodo (subject: chat.message.sent).
type chatMessageSentEvent struct {
	MessageID string `json:"message_id"`
	NodeID    string `json:"node_id"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// CreateNodeHandler maneja POST /nodes.
// Flujo:
//  1. Valida el formulario (title, description).
//  2. Genera el slug y construye el payload.
//  3. Hace un NATS Request a "node.creation.requested" y espera la decisión del Guardián.
//  4. Si el Guardián aprueba → guarda en BD y publica "node.created" para el Curador.
//  5. Si el Guardián bloquea → devuelve 409 Conflict con el motivo.
func CreateNodeHandler(db *sql.DB, nc *nats.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// ── 1. Leer y validar formulario ────────────────────────────────────────
		title := r.FormValue("title")
		description := r.FormValue("description")
		category := r.FormValue("category")
		if title == "" {
			http.Error(w, "<div class=\"error\">El título no puede estar vacío</div>", http.StatusBadRequest)
			return
		}

		if category == "" {
			category = "manga"
		}

		// ── 2. Construir payload para el Guardián ───────────────────────────────
		slug := database.GenerateSlug(title)
		payload := nodeCreationPayload{
			// NodeID temporal (antes de persistir) – suficiente para que el Guardián
			// evalúe la similitud semántica.
			NodeID:      slug, // usamos el slug como ID provisional
			Title:       title,
			Description: description,
			Category:    category,
			OwnerID:     "anonymous", // TODO: reemplazar con el user_id de la sesión JWT
			RequestedAt: time.Now().UTC().Format(time.RFC3339),
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			log.Printf("ERROR: no se pudo serializar el payload NATS: %v", err)
			http.Error(w, "<div class=\"error\">Error interno del servidor</div>", http.StatusInternalServerError)
			return
		}

		// ── 3. NATS Request → Agente Guardián ──────────────────────────────────
		var decision guardianDecision

		if nc != nil {
			msg, requestErr := nc.Request("node.creation.requested", payloadBytes, 3*time.Second)
			if requestErr != nil {
				log.Printf("WARN: el Guardián no respondió (%v); aprobando con needs_review", requestErr)
				decision = guardianDecision{Decision: "approve", NeedsReview: true}
			} else {
				if parseErr := json.Unmarshal(msg.Data, &decision); parseErr != nil {
					log.Printf("WARN: respuesta del Guardián inválida (%v); aprobando con needs_review", parseErr)
					decision = guardianDecision{Decision: "approve", NeedsReview: true}
				}
			}
		} else {
			log.Println("WARN: NATS no conectado, saltando validación del Guardián")
			decision = guardianDecision{Decision: "approve"}
		}

		// ── 4a. Guardián bloqueó la creación ────────────────────────────────────
		if decision.Decision == "block" {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusConflict)
			fmt.Fprintf(w,
				`<div class="error">❌ Nodo bloqueado por el Guardián: %s</div>`,
				decision.Reason,
			)
			return
		}

		// ── 4b. Guardián aprueba (o sugiere alternativas pero permite continuar) ─
		nodeID, err := database.CreateNode(db, title, description, category)
		if err != nil {
			log.Printf("ERROR: no se pudo crear el nodo en BD: %v", err)
			http.Error(w, fmt.Sprintf("<div class=\"error\">Error creando nodo: %v</div>", err), http.StatusInternalServerError)
			return
		}

		// ── 5. Publicar evento node.created → Agente Curador ───────────────────
		if nc != nil {
			event := nodeCreatedEvent{
				NodeID:      nodeID,
				Slug:        slug,
				Title:       title,
				Description: description,
				Category:    category,
				CreatedAt:   time.Now().UTC().Format(time.RFC3339),
			}
			eventBytes, _ := json.Marshal(event)
			if pubErr := nc.Publish("node.created", eventBytes); pubErr != nil {
				log.Printf("WARN: no se pudo publicar node.created: %v", pubErr)
			}
		}

		// ── 6. Respuesta al cliente ─────────────────────────────────────────────
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusCreated)

		if decision.NeedsReview {
			fmt.Fprintf(w, `<div class="success">✅ Nodo "<strong>%s</strong>" creado (pendiente de revisión).</div>`, title)
		} else {
			fmt.Fprintf(w, `<div class="success">✅ Nodo "<strong>%s</strong>" creado con éxito.</div>`, title)
		}
	}
}

// NodeDetailHandler maneja GET /nodes/{id}
// Muestra los detalles del nodo, el historial de chat y los resúmenes IA.
func NodeDetailHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extraer el ID de la URL: /nodes/{id}
		nodeID := strings.TrimPrefix(r.URL.Path, "/nodes/")
		nodeID = strings.TrimSuffix(nodeID, "/")
		if nodeID == "" {
			http.NotFound(w, r)
			return
		}

		node, err := database.GetNodeByID(db, nodeID)
		if err != nil {
			log.Printf("WARN: nodo no encontrado id=%s: %v", nodeID, err)
			http.NotFound(w, r)
			return
		}

		messages, err := database.ListChatMessages(db, nodeID, 100)
		if err != nil {
			log.Printf("WARN: no se pudo cargar el chat del nodo %s: %v", nodeID, err)
			messages = nil
		}

		// Cargar resúmenes IA generados por el Agente Cronista
		threads, err := database.ListAIThreadsByNodeID(db, nodeID)
		if err != nil {
			log.Printf("WARN: no se pudo cargar los hilos IA del nodo %s: %v", nodeID, err)
			threads = nil
		}

		// Detectar si hay sesión activa
		isAuthenticated := false
		username := ""
		if cookie, err := r.Cookie("nodal_session"); err == nil {
			tokenStr := strings.TrimSpace(cookie.Value)
			if claims, err := auth.ValidateToken(tokenStr); err == nil {
				isAuthenticated = true
				if user, err := database.FindUserByID(db, claims.UserID); err == nil {
					username = user.Username
				} else {
					username = "Miembro"
				}
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		views.NodeDetail(node, messages, threads, isAuthenticated, username).Render(r.Context(), w)
	}
}


// PostChatMessageHandler maneja POST /nodes/{id}/chat
// Inserta un mensaje de chat, hace broadcast vía WebSocket y devuelve 204 No Content.
func PostChatMessageHandler(db *sql.DB, nc *nats.Conn, hub *websocket.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extraer el ID de la URL: /nodes/{id}/chat
		path := strings.TrimPrefix(r.URL.Path, "/nodes/")
		path = strings.TrimSuffix(path, "/chat")
		nodeID := path
		if nodeID == "" {
			http.NotFound(w, r)
			return
		}

		content := strings.TrimSpace(r.FormValue("content"))
		if content == "" {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `<div class="chat-message"><div class="chat-message-bubble" style="color:#f87171;">El mensaje no puede estar vacío.</div></div>`)
			return
		}
		if len(content) > 500 {
			content = content[:500]
		}

		// Verificar que el nodo existe
		if _, err := database.GetNodeByID(db, nodeID); err != nil {
			http.NotFound(w, r)
			return
		}

		msg, err := database.CreateChatMessage(db, nodeID, content)
		if err != nil {
			log.Printf("ERROR: no se pudo insertar mensaje de chat en nodo %s: %v", nodeID, err)
			http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
			return
		}

		// Publicar evento al Agente Cronista vía NATS
		if nc != nil {
			event := chatMessageSentEvent{
				MessageID: msg.ID,
				NodeID:    nodeID,
				Content:   content,
				CreatedAt: msg.CreatedAt.UTC().Format(time.RFC3339),
			}
			eventBytes, _ := json.Marshal(event)
			if pubErr := nc.Publish("chat.message.sent", eventBytes); pubErr != nil {
				log.Printf("WARN: no se pudo publicar chat.message.sent: %v", pubErr)
			} else {
				log.Printf("INFO: chat.message.sent publicado — nodo=%s msg=%s", nodeID, msg.ID)
			}
		}

		// Renderizar y hacer broadcast del mensaje vía WebSocket Hub
		if hub != nil {
			var buf bytes.Buffer
			if err := views.ChatMessageBroadcast(msg).Render(r.Context(), &buf); err != nil {
				log.Printf("ERROR: error renderizando ChatMessageBroadcast: %v", err)
			} else {
				hub.Broadcast(nodeID, buf.Bytes())
			}
		}

		// Retornar 204 No Content ya que la actualización de la UI se realiza vía WS
		w.WriteHeader(http.StatusNoContent)
	}
}

var upgrader = gorilla.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Permitir todas las conexiones para desarrollo local
	},
}

// WebSocketHandler maneja la actualización del protocolo de HTTP a WS.
func WebSocketHandler(hub *websocket.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extraer el ID de la URL: /nodes/{id}/ws
		path := strings.TrimPrefix(r.URL.Path, "/nodes/")
		path = strings.TrimSuffix(path, "/ws")
		nodeID := path
		if nodeID == "" {
			http.Error(w, "Node ID required", http.StatusBadRequest)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WS: Upgrade failed for node %s: %v", nodeID, err)
			return
		}

		client := &websocket.Client{
			Hub:    hub,
			Conn:   conn,
			Send:   make(chan []byte, 256),
			NodeID: nodeID,
		}

		hub.Register(client)

		go client.WritePump()
		go client.ReadPump()
	}
}

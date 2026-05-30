package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	gorilla "github.com/gorilla/websocket"
	nats "github.com/nats-io/nats.go"

	"nodal/internal/auth"
	"nodal/internal/handlers/views"
	"nodal/internal/middleware"
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
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			log.Printf("WARN: no se pudo parsear multipart form: %v", err)
		}

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

		var imageURL *string
		file, header, err := r.FormFile("image")
		if err == nil {
			defer file.Close()
			uploadsDir := "./static/uploads"
			if err := os.MkdirAll(uploadsDir, os.ModePerm); err != nil {
				log.Printf("ERROR: no se pudo crear el directorio de subidas: %v", err)
				http.Error(w, "<div class=\"error\">Error interno del servidor</div>", http.StatusInternalServerError)
				return
			}

			ext := filepath.Ext(header.Filename)
			newFilename := fmt.Sprintf("img-%d%s", time.Now().UnixNano(), ext)
			filePath := filepath.Join(uploadsDir, newFilename)

			out, err := os.Create(filePath)
			if err != nil {
				log.Printf("ERROR: no se pudo crear el archivo en disco: %v", err)
				http.Error(w, "<div class=\"error\">Error guardando la imagen</div>", http.StatusInternalServerError)
				return
			}
			defer out.Close()

			if _, err := io.Copy(out, file); err != nil {
				log.Printf("ERROR: no se pudo copiar el archivo: %v", err)
				http.Error(w, "<div class=\"error\">Error al escribir la imagen</div>", http.StatusInternalServerError)
				return
			}

			urlPath := "/static/uploads/" + newFilename
			imageURL = &urlPath
		} else if err != http.ErrMissingFile {
			log.Printf("WARN: error al obtener el archivo image: %v", err)
		}

		var ownerID string = "anonymous"
		claims := middleware.ClaimsFromContext(r.Context())
		if claims != nil {
			ownerID = claims.UserID
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
			OwnerID:     ownerID,
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
		var ownerIDPtr *string
		if ownerID != "anonymous" {
			ownerIDPtr = &ownerID
		}
		nodeID, err := database.CreateNode(db, title, description, category, imageURL, ownerIDPtr)
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
		nodeID := r.PathValue("id")
		if nodeID == "" {
			http.NotFound(w, r)
			return
		}

		var userID *string
		if cookie, err := r.Cookie("nodal_session"); err == nil {
			tokenStr := strings.TrimSpace(cookie.Value)
			if claims, err := auth.ValidateToken(tokenStr); err == nil {
				userID = &claims.UserID
			}
		}

		node, err := database.GetNodeByID(db, nodeID, userID)
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
		nodeID := r.PathValue("id")
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
		if _, err := database.GetNodeByID(db, nodeID, nil); err != nil {
			http.NotFound(w, r)
			return
		}

		var userID *string
		claims := middleware.ClaimsFromContext(r.Context())
		if claims != nil {
			userID = &claims.UserID
		} else {
			if cookie, err := r.Cookie("nodal_session"); err == nil {
				tokenStr := strings.TrimSpace(cookie.Value)
				if cl, err := auth.ValidateToken(tokenStr); err == nil {
					userID = &cl.UserID
				}
			}
		}

		parentIDStr := r.FormValue("parent_id")
		var parentID *string
		if parentIDStr != "" {
			parentID = &parentIDStr
		}

		msg, err := database.CreateChatMessage(db, nodeID, content, userID, parentID)
		if err != nil {
			log.Printf("ERROR Guardando mensaje: %v", err)
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
		nodeID := r.PathValue("id")
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

// NodeLikeHandler handles POST /nodes/{id}/like
func NodeLikeHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		nodeID := r.PathValue("id")
		if nodeID == "" {
			http.NotFound(w, r)
			return
		}

		claims := middleware.ClaimsFromContext(r.Context())
		if claims == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		userID := claims.UserID

		_, err := database.ToggleLike(db, userID, nodeID)
		if err != nil {
			log.Printf("ERROR: failed to toggle like for node %s user %s: %v", nodeID, userID, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Retrieve updated node with updated like count and is_liked status
		updatedNode, err := database.GetNodeByID(db, nodeID, &userID)
		if err != nil {
			log.Printf("ERROR: failed to get updated node %s: %v", nodeID, err)
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		views.NodeCard(*updatedNode).Render(r.Context(), w)
	}
}

// ToggleSaveHandler handles POST /nodes/{id}/save and returns ONLY the SaveButton component.
func ToggleSaveHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		nodeID := r.PathValue("id")
		if nodeID == "" {
			http.NotFound(w, r)
			return
		}

		claims := middleware.ClaimsFromContext(r.Context())
		if claims == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		userID := claims.UserID

		isSaved, err := database.ToggleSaveNode(db, userID, nodeID)
		if err != nil {
			log.Printf("ERROR: failed to toggle save for node %s user %s: %v", nodeID, userID, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		views.SaveButton(nodeID, isSaved).Render(r.Context(), w)
	}
}

// SearchHandler handles GET /search.
// Reads "q" parameter. If empty, list all nodes. Otherwise search.
func SearchHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		q = strings.TrimPrefix(q, "#") // normalizar hashtags: #OnePiece → OnePiece
		

		var userID *string
		if cookie, err := r.Cookie("nodal_session"); err == nil {
			tokenStr := strings.TrimSpace(cookie.Value)
			if claims, err := auth.ValidateToken(tokenStr); err == nil {
				userID = &claims.UserID
			}
		}

		var nodes []database.Node
		var err error
		if q == "" {
			nodes, err = database.ListNodes(db, userID)
		} else {
			nodes, err = database.SearchNodes(db, q, userID)
		}

		if err != nil {
			log.Printf("ERROR: search nodes failed: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if len(nodes) == 0 {
			w.Write([]byte(`<div class="nodes-empty"><p>No se encontraron nodos. 🔍</p></div>`))
			return
		}

		for _, n := range nodes {
			views.NodeCard(n).Render(r.Context(), w)
		}
	}
}

// LeftSidebarHandler handles GET /components/sidebar/left.
// Returns only Navigation + Top-5 trending hashtags (no categories).
func LeftSidebarHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract trending hashtags from all nodes via Regex
		nodes, err := database.ListNodes(db, nil)
		var trends []database.Trend
		if err == nil {
			counts := make(map[string]int)
			canonical := make(map[string]string)
			tagRegexp := regexp.MustCompile(`#\w+`)

			for _, node := range nodes {
				text := node.Title + " " + node.Description
				matches := tagRegexp.FindAllString(text, -1)
				for _, match := range matches {
					cleanTag := strings.TrimPrefix(match, "#")
					lowerTag := strings.ToLower(cleanTag)
					counts[lowerTag]++
					if _, exists := canonical[lowerTag]; !exists {
						canonical[lowerTag] = cleanTag
					}
				}
			}

			for lowerTag, count := range counts {
				trends = append(trends, database.Trend{
					Name:  canonical[lowerTag],
					Count: count,
				})
			}

			// Sort by count descending, then alphabetically
			sort.Slice(trends, func(i, j int) bool {
				if trends[i].Count == trends[j].Count {
					return trends[i].Name < trends[j].Name
				}
				return trends[i].Count > trends[j].Count
			})

			if len(trends) > 5 {
				trends = trends[:5]
			}
		} else {
			log.Printf("WARN: failed to get nodes for trends in left sidebar: %v", err)
		}

		// Fallback mock topics if no hashtags found in DB
		if len(trends) == 0 {
			trends = []database.Trend{
				{Name: "Valorant", Count: 3},
				{Name: "Pokemon", Count: 2},
				{Name: "OnePiece", Count: 2},
				{Name: "IA", Count: 2},
				{Name: "Dev", Count: 1},
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		views.LeftSidebarContent(trends).Render(r.Context(), w)
	}
}

// RightSidebarHandler handles GET /components/sidebar/right.
// Returns compact popular nodes list and active trending topics.
func RightSidebarHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		popularNodes, err := database.GetPopularNodes(db, 4)
		if err != nil {
			log.Printf("ERROR: failed to get popular nodes: %v", err)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(`<aside class="widgets-sidebar flex flex-col gap-6 w-80 flex-shrink-0 hidden lg:block"><div class="widget-box w-full"><div style="color: var(--text-muted); font-size: var(--font-size-xs); padding: var(--space-2);">Error cargando comunidades</div></div></aside>`))
			return
		}

		trends, err := database.GetTrendingTopics(db, 4)
		if err != nil {
			log.Printf("WARN: failed to get trending topics for right sidebar: %v", err)
			trends = []database.TrendingTopic{}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		views.RightSidebarContent(popularNodes, trends).Render(r.Context(), w)
	}
}



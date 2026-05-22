package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	nats "github.com/nats-io/nats.go"

	"nodal/internal/platform/database"
)

// nodeCreationPayload es el payload que se envía al Agente Guardián
// vía NATS Request-Reply (subject: node.creation.requested).
type nodeCreationPayload struct {
	NodeID      string `json:"node_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
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
	CreatedAt   string `json:"created_at"`
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

		if title == "" {
			http.Error(w, "<div class=\"error\">El título no puede estar vacío</div>", http.StatusBadRequest)
			return
		}

		// ── 2. Construir payload para el Guardián ───────────────────────────────
		slug := database.GenerateSlug(title)
		payload := nodeCreationPayload{
			// NodeID temporal (antes de persistir) – suficiente para que el Guardián
			// evalúe la similitud semántica.
			NodeID:      slug, // usamos el slug como ID provisional
			Title:       title,
			Description: description,
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
		// Timeout de 3 s: si el Guardián no responde, aprobamos con needs_review=true
		// para no bloquear al usuario (PRD R3 – degradación elegante).
		var decision guardianDecision

		if nc != nil {
			msg, requestErr := nc.Request("node.creation.requested", payloadBytes, 3*time.Second)
			if requestErr != nil {
				// Timeout o error de transporte → degradación elegante
				log.Printf("WARN: el Guardián no respondió (%v); aprobando con needs_review", requestErr)
				decision = guardianDecision{Decision: "approve", NeedsReview: true}
			} else {
				if parseErr := json.Unmarshal(msg.Data, &decision); parseErr != nil {
					log.Printf("WARN: respuesta del Guardián inválida (%v); aprobando con needs_review", parseErr)
					decision = guardianDecision{Decision: "approve", NeedsReview: true}
				}
			}
		} else {
			// NATS no configurado (entorno de desarrollo sin NATS) → aprobación directa
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
		if err := database.CreateNode(db, title, description); err != nil {
			log.Printf("ERROR: no se pudo crear el nodo en BD: %v", err)
			http.Error(w, fmt.Sprintf("<div class=\"error\">Error creando nodo: %v</div>", err), http.StatusInternalServerError)
			return
		}

		// ── 5. Publicar evento node.created → Agente Curador ───────────────────
		if nc != nil {
			event := nodeCreatedEvent{
				NodeID:      slug,
				Slug:        slug,
				Title:       title,
				Description: description,
				CreatedAt:   time.Now().UTC().Format(time.RFC3339),
			}
			eventBytes, _ := json.Marshal(event)
			if pubErr := nc.Publish("node.created", eventBytes); pubErr != nil {
				// No es crítico: el nodo ya fue guardado; el Curador lo enriquecerá
				// en cuanto NATS vuelva a estar disponible.
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

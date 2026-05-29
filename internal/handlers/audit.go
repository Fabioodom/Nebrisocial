package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strings"

	"nodal/internal/auth"
	"nodal/internal/handlers/views"
	"nodal/internal/platform/database"
)

// AuditHandler maneja GET /admin/audit.
// Recupera los últimos 200 registros del log de auditoría de agentes IA
// y los renderiza mediante el template AuditDashboard.
func AuditHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		logs, err := database.ListAuditLogs(db, 200)
		if err != nil {
			log.Printf("ERROR: no se pudieron cargar los logs de auditoría: %v", err)
			http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
			return
		}

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
		views.AuditDashboard(logs, isAuthenticated, username).Render(r.Context(), w)
	}
}


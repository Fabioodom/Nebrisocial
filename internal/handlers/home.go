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

// HomeHandler renderiza la página principal.
// Detecta si el usuario tiene un access token en la cookie 'nodal_session'
// y, si es válido, pasa isAuthenticated=true y el username al template.
// También carga la lista de nodos para mostrarla en la página.
func HomeHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Solo atendemos exactamente "/"
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		isAuthenticated := false
		username := ""

		// Intentar leer el access token de la cookie de sesión de navegador
		if cookie, err := r.Cookie("nodal_session"); err == nil {
			tokenStr := strings.TrimSpace(cookie.Value)
			if claims, err := auth.ValidateToken(tokenStr); err == nil {
				isAuthenticated = true
				if user, err := database.FindUserByID(db, claims.UserID); err == nil {
					username = user.Username
				} else {
					username = "Miembro"
				}
				_ = claims
			}
		}

		// Cargar lista de nodos (siempre, para usuarios autenticados y no autenticados)
		nodes, err := database.ListNodes(db)
		if err != nil {
			log.Printf("WARN: no se pudo cargar la lista de nodos: %v", err)
			nodes = nil
		}

		views.Home(isAuthenticated, username, nodes).Render(r.Context(), w)
	}
}

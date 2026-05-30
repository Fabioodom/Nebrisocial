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
		var userID *string

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
				userID = &claims.UserID
			}
		}

		// Cargar lista de nodos (personalizado si está autenticado, genérico si no)
		var nodes []database.Node
		var err error

		if isAuthenticated && userID != nil {
			nodes, err = database.GetPersonalizedFeed(db, *userID, 20)
			if err != nil {
				log.Printf("WARN: no se pudo cargar el feed personalizado para el usuario %s: %v. Revirtiendo a feed genérico.", *userID, err)
				nodes, err = database.ListNodes(db, userID)
			}
		} else {
			nodes, err = database.ListNodes(db, userID)
		}

		if err != nil {
			log.Printf("WARN: no se pudo cargar la lista de nodos: %v", err)
			nodes = nil
		}

		views.Home(isAuthenticated, username, nodes).Render(r.Context(), w)
	}
}


// ExploreHandler renderiza la página de exploración.
func ExploreHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		isAuthenticated := false
		username := ""
		var userID *string

		if cookie, err := r.Cookie("nodal_session"); err == nil {
			tokenStr := strings.TrimSpace(cookie.Value)
			if claims, err := auth.ValidateToken(tokenStr); err == nil {
				isAuthenticated = true
				if user, err := database.FindUserByID(db, claims.UserID); err == nil {
					username = user.Username
				} else {
					username = "Miembro"
				}
				userID = &claims.UserID
			}
		}

		q := strings.TrimSpace(r.URL.Query().Get("q"))
		q = strings.TrimPrefix(q, "#") // normalizar hashtags: #OnePiece → OnePiece
		var nodes []database.Node
		var err error

		if q != "" {
			nodes, err = database.SearchNodes(db, q, userID)

			if err != nil {
				log.Printf("WARN: no se pudo buscar nodos para explorar con query %q: %v", q, err)
				nodes = nil
			}
		} else {
			nodes, err = database.ListNodes(db, userID)
			if err != nil {
				log.Printf("WARN: no se pudo cargar la lista de nodos para explorar: %v", err)
				nodes = nil
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		views.Explore(isAuthenticated, username, nodes).Render(r.Context(), w)
	}
}



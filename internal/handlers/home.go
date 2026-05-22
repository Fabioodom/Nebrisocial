package handlers

import (
	"net/http"
	"strings"

	"nodal/internal/auth"
	"nodal/internal/handlers/views"
)

// HomeHandler renderiza la página principal.
// Detecta si el usuario tiene un access token en la cookie 'nodal_access_token'
// y, si es válido, pasa isAuthenticated=true y el username al template.
func HomeHandler(w http.ResponseWriter, r *http.Request) {
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
			username = claims.UserID // Temporalmente usamos el ID; se puede enriquecer con FindUserByID
			_ = claims
		}
	}

	views.Home(isAuthenticated, username).Render(r.Context(), w)
}

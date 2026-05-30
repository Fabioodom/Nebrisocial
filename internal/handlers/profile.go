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

// ProfileHandler renderiza la página de perfil (propia o ajena).
func ProfileHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		isAuthenticated := false
		currentUsername := ""
		var currentUserID string

		// Validar sesión del usuario autenticado
		if cookie, err := r.Cookie("nodal_session"); err == nil {
			tokenStr := strings.TrimSpace(cookie.Value)
			if claims, err := auth.ValidateToken(tokenStr); err == nil {
				isAuthenticated = true
				if user, err := database.FindUserByID(db, claims.UserID); err == nil {
					currentUsername = user.Username
				} else {
					currentUsername = "Miembro"
				}
				currentUserID = claims.UserID
			}
		}

		if !isAuthenticated {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Obtener ID o username del perfil a consultar
		targetUserID := r.URL.Query().Get("id")
		targetUsername := r.URL.Query().Get("username")

		var targetUser *database.User
		var err error

		if targetUserID != "" {
			targetUser, err = database.FindUserByID(db, targetUserID)
		} else if targetUsername != "" {
			targetUser, err = database.FindUserByUsername(db, targetUsername)
		} else {
			targetUser, err = database.FindUserByID(db, currentUserID)
		}

		if err != nil {
			log.Printf("WARN: no se encontró al usuario (ID: %s, Username: %s): %v", targetUserID, targetUsername, err)
			http.Error(w, "Usuario no encontrado", http.StatusNotFound)
			return
		}

		targetUserID = targetUser.ID
		isOwnProfile := targetUserID == currentUserID

		// Obtener estadísticas de seguimiento
		stats, err := database.GetProfileStats(db, targetUserID, currentUserID)
		if err != nil {
			log.Printf("WARN: no se pudieron obtener estadísticas para el usuario %s: %v", targetUserID, err)
			stats = &database.ProfileStats{FollowersCount: 0, FollowingCount: 0, IsFollowing: false}
		}

		var ownedNodes []database.Node
		var savedNodes []database.Node

		// Control de privacidad: sólo cargar nodos si es público, es propio, o se le sigue
		canViewContent := isOwnProfile || !targetUser.IsPrivate || stats.IsFollowing

		if canViewContent {
			ownedNodes, err = database.ListNodesByOwner(db, targetUserID)
			if err != nil {
				log.Printf("WARN: no se pudieron cargar los nodos creados por %s: %v", targetUserID, err)
				ownedNodes = nil
			}

			// Nodos guardados sólo se muestran en el perfil propio por privacidad de favoritos
			if isOwnProfile {
				savedNodes, err = database.ListSavedNodes(db, targetUserID)
				if err != nil {
					log.Printf("WARN: no se pudieron cargar los nodos guardados por %s: %v", targetUserID, err)
					savedNodes = nil
				}
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		views.Profile(isAuthenticated, currentUsername, targetUser, stats, isOwnProfile, ownedNodes, savedNodes).Render(r.Context(), w)
	}
}

// EditProfileHandler procesa el formulario de edición de perfil.
func EditProfileHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var currentUserID string
		if cookie, err := r.Cookie("nodal_session"); err == nil {
			tokenStr := strings.TrimSpace(cookie.Value)
			if claims, err := auth.ValidateToken(tokenStr); err == nil {
				currentUserID = claims.UserID
			}
		}

		if currentUserID == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		bio := r.FormValue("bio")
		isPrivate := r.FormValue("is_private") == "on"

		err := database.UpdateProfile(db, currentUserID, bio, isPrivate)
		if err != nil {
			log.Printf("ERROR: no se pudo actualizar el perfil del usuario %s: %v", currentUserID, err)
			http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/profile", http.StatusSeeOther)
	}
}

// FollowHandler procesa la acción de Seguir/Dejar de seguir de forma asíncrona vía HTMX.
func FollowHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var currentUserID string
		if cookie, err := r.Cookie("nodal_session"); err == nil {
			tokenStr := strings.TrimSpace(cookie.Value)
			if claims, err := auth.ValidateToken(tokenStr); err == nil {
				currentUserID = claims.UserID
			}
		}

		if currentUserID == "" {
			w.Header().Set("HX-Redirect", "/login")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		targetUserID := r.PathValue("id")
		if targetUserID == "" || targetUserID == currentUserID {
			http.Error(w, "Acción inválida", http.StatusBadRequest)
			return
		}

		followStatus, err := database.ToggleFollow(db, currentUserID, targetUserID)
		if err != nil {
			log.Printf("ERROR: fallo en ToggleFollow (seguidor: %s, seguido: %s): %v", currentUserID, targetUserID, err)
			http.Error(w, "Error interno", http.StatusInternalServerError)
			return
		}

		// Obtener estadísticas de seguimiento actualizadas
		stats, err := database.GetProfileStats(db, targetUserID, currentUserID)
		if err != nil {
			log.Printf("WARN: no se pudieron obtener estadísticas actualizadas para el usuario %s: %v", targetUserID, err)
			stats = &database.ProfileStats{FollowersCount: 0, FollowingCount: 0, IsFollowing: false, FollowStatus: followStatus}
		}

		targetUser, err := database.FindUserByID(db, targetUserID)
		if err != nil {
			log.Printf("WARN: no se pudo obtener el usuario de destino %s: %v", targetUserID, err)
			http.Error(w, "Usuario no encontrado", http.StatusNotFound)
			return
		}

		isOwnProfile := targetUserID == currentUserID

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		views.ProfileStatsAndActions(targetUser, stats, isOwnProfile).Render(r.Context(), w)
	}
}


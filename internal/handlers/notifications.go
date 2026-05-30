package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"

	"nodal/internal/handlers/views"
	"nodal/internal/auth"
	"nodal/internal/platform/database"
)

// NotificationsHandler renderiza la bandeja de notificaciones.
// Si la petición viene de HTMX, devuelve únicamente la lista de notificaciones parcial.
func NotificationsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		isAuthenticated := false
		currentUsername := ""
		var currentUserID string

		// Validar sesión
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
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/login")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Obtener las notificaciones del usuario
		notifications, err := database.ListNotifications(db, currentUserID)
		if err != nil {
			log.Printf("ERROR: no se pudieron cargar las notificaciones para %s: %v", currentUserID, err)
			notifications = nil
		}

		// Marcar notificaciones como leídas
		err = database.MarkNotificationsAsRead(db, currentUserID)
		if err != nil {
			log.Printf("WARN: no se pudieron marcar las notificaciones como leídas para %s: %v", currentUserID, err)
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Header.Get("HX-Request") == "true" {
			views.NotificationsList(notifications).Render(r.Context(), w)
		} else {
			views.NotificationsPage(isAuthenticated, currentUsername, notifications).Render(r.Context(), w)
		}
	}
}

// NotificationsAcceptHandler acepta una solicitud de seguimiento.
func NotificationsAcceptHandler(db *sql.DB) http.HandlerFunc {
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

		notificationID := r.PathValue("id")
		if notificationID == "" {
			http.Error(w, "ID de notificación inválido", http.StatusBadRequest)
			return
		}

		err := database.AcceptFollowRequestByID(db, notificationID, currentUserID)
		if err != nil {
			log.Printf("ERROR: fallo al aceptar solicitud de seguimiento por ID (notif: %s, usuario: %s): %v", notificationID, currentUserID, err)
			http.Error(w, "Error interno", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// NotificationsRejectHandler rechaza una solicitud de seguimiento.
func NotificationsRejectHandler(db *sql.DB) http.HandlerFunc {
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

		notificationID := r.PathValue("id")
		if notificationID == "" {
			http.Error(w, "ID de notificación inválido", http.StatusBadRequest)
			return
		}

		err := database.RejectFollowRequestByID(db, notificationID, currentUserID)
		if err != nil {
			log.Printf("ERROR: fallo al rechazar solicitud de seguimiento por ID (notif: %s, usuario: %s): %v", notificationID, currentUserID, err)
			http.Error(w, "Error interno", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// NotificationsUnreadCountHandler devuelve el badge de notificaciones no leídas si hay alguna.
func NotificationsUnreadCountHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var currentUserID string
		if cookie, err := r.Cookie("nodal_session"); err == nil {
			tokenStr := strings.TrimSpace(cookie.Value)
			if claims, err := auth.ValidateToken(tokenStr); err == nil {
				currentUserID = claims.UserID
			}
		}

		if currentUserID == "" {
			w.WriteHeader(http.StatusOK)
			return
		}

		var unreadCount int
		err := db.QueryRow(`
			SELECT COUNT(*) FROM notifications
			WHERE user_id = $1 AND is_read = false
		`, currentUserID).Scan(&unreadCount)
		if err != nil {
			log.Printf("ERROR: no se pudo obtener conteo de notificaciones no leídas para %s: %v", currentUserID, err)
			w.WriteHeader(http.StatusOK)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if unreadCount > 0 {
			countStr := fmt.Sprintf("%d", unreadCount)
			if unreadCount > 99 {
				countStr = "99+"
			}
			fmt.Fprintf(w, `<span id="notif-badge" class="notif-badge" style="position: absolute; top: -4px; right: -4px; background: var(--brand-color); color: white; border-radius: 50%; width: 16px; height: 16px; font-size: 10px; display: flex; align-items: center; justify-content: center; font-weight: bold; border: 2px solid var(--bg-surface);">%s</span>`, countStr)
		} else {
			w.Write([]byte(`<span id="notif-badge"></span>`))
		}
	}
}

// NotificationsAllHandler devuelve el historial completo de notificaciones para el modal.
// No marca las notificaciones como leídas (eso se hace en el handler principal).
func NotificationsAllHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
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

		notifications, err := database.ListNotifications(db, currentUserID)
		if err != nil {
			log.Printf("ERROR: no se pudieron cargar todas las notificaciones para %s: %v", currentUserID, err)
			notifications = nil
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		views.NotificationsModalContent(notifications).Render(r.Context(), w)
	}
}


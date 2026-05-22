// Package handlers contiene los manejadores HTTP de Nodal.
// auth.go implementa los endpoints de registro, login, refresh, logout y vistas GET.
package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"nodal/internal/auth"
	"nodal/internal/handlers/views"
	"nodal/internal/platform/crypto"
	"nodal/internal/platform/database"
)

// ── Estructuras de request/response ──────────────────────────────────────────

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

// errorResponse es el formato JSON estándar para errores de la API.
type errorResponse struct {
	Error string `json:"error"`
}

// ── Detección de entorno ───────────────────────────────────────────────────────

// isDevMode devuelve true cuando APP_ENV != "production".
// En modo dev las cookies NO se marcan como Secure (para HTTP/localhost).
func isDevMode() bool {
	return os.Getenv("APP_ENV") != "production"
}

// ── Helper de respuestas ──────────────────────────────────────────────────────

// writeJSON serializa v como JSON con el status code dado.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf(`{"level":"error","handler":"writeJSON","msg":"fallo al serializar respuesta"}`)
	}
}

// writeError envía un error en JSON.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

// writeHTMLError escribe un fragmento HTML con el mensaje de error.
// htmx inyectará este fragmento en el div #auth-error.
func writeHTMLError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	w.Write([]byte(msg))
}

// isHTMX devuelve true si la request viene de htmx (header HX-Request presente).
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// parseFormOrJSON lee los campos de la request ya sea desde
// application/x-www-form-urlencoded (formularios htmx) o desde application/json.
func parseFormFields(r *http.Request) (username, email, password string) {
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		var body struct {
			Username string `json:"username"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		return body.Username, body.Email, body.Password
	}
	// Form-urlencoded (htmx o <form> estándar)
	r.ParseForm()
	return r.FormValue("username"), r.FormValue("email"), r.FormValue("password")
}

// ── Cookies ───────────────────────────────────────────────────────────────────

const (
	refreshTokenCookieName = "nodal_refresh_token"
	sessionCookieName      = "nodal_session" // access token para el navegador
)

// setRefreshCookie guarda el refresh token en una cookie HttpOnly.
func setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    token,
		Path:     "/auth",
		HttpOnly: true,
		Secure:   !isDevMode(), // false en localhost, true en producción
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
	})
}

// setSessionCookie guarda el access token en una cookie accesible en todo el sitio.
// Permite que el HomeHandler detecte la sesión activa en GET /.
// Path "/" para que se envíe en todas las rutas.
func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   !isDevMode(),
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(auth.AccessTokenTTL.Seconds()),
	})
}

// clearRefreshCookie elimina la cookie de refresh token.
func clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    "",
		Path:     "/auth",
		HttpOnly: true,
		Secure:   !isDevMode(),
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

// clearSessionCookie elimina la cookie de sesión del navegador.
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   !isDevMode(),
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

// ── AuthHandler ───────────────────────────────────────────────────────────────

// AuthHandler provee los manejadores HTTP para el flujo de autenticación.
type AuthHandler struct {
	db *sql.DB
}

// NewAuthHandler crea un AuthHandler con la BD inyectada.
func NewAuthHandler(db *sql.DB) *AuthHandler {
	return &AuthHandler{db: db}
}

// ── GET /login ────────────────────────────────────────────────────────────────

// ShowLogin renderiza la página de inicio de sesión.
func (h *AuthHandler) ShowLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	// Si ya está autenticado, redirigir a home
	if _, err := r.Cookie(sessionCookieName); err == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	views.Login().Render(r.Context(), w)
}

// ── GET /register ─────────────────────────────────────────────────────────────

// ShowRegister renderiza la página de registro.
func (h *AuthHandler) ShowRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}
	if _, err := r.Cookie(sessionCookieName); err == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	views.Register().Render(r.Context(), w)
}

// ── POST /auth/register ───────────────────────────────────────────────────────

// Register maneja el registro de nuevos usuarios.
// Acepta tanto application/json como application/x-www-form-urlencoded.
// Si viene de htmx, responde con HX-Redirect para redirigir al navegador a /.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}

	username, email, password := parseFormFields(r)

	if username == "" || email == "" || password == "" {
		if isHTMX(r) {
			writeHTMLError(w, http.StatusUnprocessableEntity, "Username, email y contraseña son obligatorios.")
			return
		}
		writeError(w, http.StatusUnprocessableEntity, "username, email y password son obligatorios")
		return
	}
	if len(password) < 8 {
		if isHTMX(r) {
			writeHTMLError(w, http.StatusUnprocessableEntity, "La contraseña debe tener al menos 8 caracteres.")
			return
		}
		writeError(w, http.StatusUnprocessableEntity, "la contraseña debe tener al menos 8 caracteres")
		return
	}

	hash, err := crypto.HashPassword(password)
	if err != nil {
		log.Printf(`{"level":"error","handler":"Register","msg":"fallo al hashear contraseña"}`)
		if isHTMX(r) {
			writeHTMLError(w, http.StatusInternalServerError, "Error interno del servidor. Inténtalo de nuevo.")
			return
		}
		writeError(w, http.StatusInternalServerError, "error interno del servidor")
		return
	}

	user, err := database.CreateUser(h.db, username, email, hash)
	if err != nil {
		if errors.Is(err, database.ErrEmailAlreadyExists) {
			if isHTMX(r) {
				writeHTMLError(w, http.StatusConflict, "Este email ya está registrado. ¿Quieres <a href=\"/login\">iniciar sesión</a>?")
				return
			}
			writeError(w, http.StatusConflict, "el email ya está registrado")
			return
		}
		log.Printf(`{"level":"error","handler":"Register","msg":"error al crear usuario en BD"}`)
		if isHTMX(r) {
			writeHTMLError(w, http.StatusInternalServerError, "Error interno del servidor. Inténtalo de nuevo.")
			return
		}
		writeError(w, http.StatusInternalServerError, "error interno del servidor")
		return
	}

	pair, err := auth.GenerateTokenPair(user.ID, "member")
	if err != nil {
		log.Printf(`{"level":"error","handler":"Register","msg":"error al generar tokens JWT"}`)
		if isHTMX(r) {
			writeHTMLError(w, http.StatusInternalServerError, "Error al generar sesión. Inténtalo de nuevo.")
			return
		}
		writeError(w, http.StatusInternalServerError, "error interno del servidor")
		return
	}

	setRefreshCookie(w, pair.RefreshToken)
	setSessionCookie(w, pair.AccessToken)

	// Respuesta htmx: redirigir al navegador a / mediante header HX-Redirect
	if isHTMX(r) {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)
		return
	}

	writeJSON(w, http.StatusCreated, tokenResponse{
		AccessToken: pair.AccessToken,
		TokenType:   "Bearer",
	})
}

// ── POST /auth/login ──────────────────────────────────────────────────────────

// Login autentica a un usuario existente.
// Acepta tanto JSON como form-urlencoded. Si viene de htmx, usa HX-Redirect.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}

	_, email, password := parseFormFields(r)

	if email == "" || password == "" {
		if isHTMX(r) {
			writeHTMLError(w, http.StatusUnprocessableEntity, "Email y contraseña son obligatorios.")
			return
		}
		writeError(w, http.StatusUnprocessableEntity, "email y password son obligatorios")
		return
	}

	user, err := database.FindUserByEmail(h.db, email)
	if err != nil {
		if isHTMX(r) {
			writeHTMLError(w, http.StatusUnauthorized, "Credenciales inválidas. Revisa tu email y contraseña.")
			return
		}
		writeError(w, http.StatusUnauthorized, "credenciales inválidas")
		return
	}

	if !crypto.CheckPassword(password, user.PasswordHash) {
		if isHTMX(r) {
			writeHTMLError(w, http.StatusUnauthorized, "Credenciales inválidas. Revisa tu email y contraseña.")
			return
		}
		writeError(w, http.StatusUnauthorized, "credenciales inválidas")
		return
	}

	pair, err := auth.GenerateTokenPair(user.ID, "member")
	if err != nil {
		log.Printf(`{"level":"error","handler":"Login","msg":"error al generar tokens JWT","user_id":"%s"}`, user.ID)
		if isHTMX(r) {
			writeHTMLError(w, http.StatusInternalServerError, "Error al generar sesión. Inténtalo de nuevo.")
			return
		}
		writeError(w, http.StatusInternalServerError, "error interno del servidor")
		return
	}

	setRefreshCookie(w, pair.RefreshToken)
	setSessionCookie(w, pair.AccessToken)

	if isHTMX(r) {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)
		return
	}

	writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken: pair.AccessToken,
		TokenType:   "Bearer",
	})
}

// ── POST /auth/refresh ────────────────────────────────────────────────────────

// Refresh valida el refresh token y emite un nuevo access token.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}

	cookie, err := r.Cookie(refreshTokenCookieName)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "refresh token no encontrado")
		return
	}

	claims, err := auth.ValidateToken(cookie.Value)
	if err != nil {
		clearRefreshCookie(w)
		clearSessionCookie(w)
		writeError(w, http.StatusUnauthorized, "refresh token inválido o expirado")
		return
	}

	_, err = database.FindUserByID(h.db, claims.UserID)
	if err != nil {
		clearRefreshCookie(w)
		clearSessionCookie(w)
		writeError(w, http.StatusUnauthorized, "usuario no encontrado")
		return
	}

	pair, err := auth.GenerateTokenPair(claims.UserID, claims.Role)
	if err != nil {
		log.Printf(`{"level":"error","handler":"Refresh","msg":"error al generar tokens JWT"}`)
		writeError(w, http.StatusInternalServerError, "error interno del servidor")
		return
	}

	setRefreshCookie(w, pair.RefreshToken)
	setSessionCookie(w, pair.AccessToken)
	writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken: pair.AccessToken,
		TokenType:   "Bearer",
	})
}

// ── POST /auth/logout ─────────────────────────────────────────────────────────

// Logout elimina las cookies de sesión y refresh token.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}

	clearRefreshCookie(w)
	clearSessionCookie(w)

	if isHTMX(r) {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "sesión cerrada correctamente"})
}

// ── Handlers OAuth2 ───────────────────────────────────────────────────────────

// GoogleLogin inicia el flujo OAuth2 con Google.
func (h *AuthHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := auth.GenerateOAuthState()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "error al generar state OAuth")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/auth",
		HttpOnly: true,
		Secure:   !isDevMode(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})

	http.Redirect(w, r, auth.GetGoogleAuthURL(state), http.StatusTemporaryRedirect)
}

// GoogleCallback procesa el callback de Google.
func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		writeError(w, http.StatusBadRequest, "state OAuth inválido (posible ataque CSRF)")
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "code OAuth ausente")
		return
	}

	user, err := auth.HandleGoogleCallback(r.Context(), h.db, code)
	if err != nil {
		log.Printf(`{"level":"error","handler":"GoogleCallback","msg":"fallo en callback de Google"}`)
		writeError(w, http.StatusInternalServerError, "error al autenticar con Google")
		return
	}

	pair, err := auth.GenerateTokenPair(user.ID, "member")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "error al generar tokens JWT")
		return
	}

	setRefreshCookie(w, pair.RefreshToken)
	setSessionCookie(w, pair.AccessToken)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// GitHubLogin inicia el flujo OAuth2 con GitHub.
func (h *AuthHandler) GitHubLogin(w http.ResponseWriter, r *http.Request) {
	state, err := auth.GenerateOAuthState()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "error al generar state OAuth")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/auth",
		HttpOnly: true,
		Secure:   !isDevMode(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})

	http.Redirect(w, r, auth.GetGitHubAuthURL(state), http.StatusTemporaryRedirect)
}

// GitHubCallback procesa el callback de GitHub.
func (h *AuthHandler) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		writeError(w, http.StatusBadRequest, "state OAuth inválido (posible ataque CSRF)")
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "code OAuth ausente")
		return
	}

	user, err := auth.HandleGitHubCallback(r.Context(), h.db, code)
	if err != nil {
		log.Printf(`{"level":"error","handler":"GitHubCallback","msg":"fallo en callback de GitHub"}`)
		writeError(w, http.StatusInternalServerError, "error al autenticar con GitHub")
		return
	}

	pair, err := auth.GenerateTokenPair(user.ID, "member")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "error al generar tokens JWT")
		return
	}

	setRefreshCookie(w, pair.RefreshToken)
	setSessionCookie(w, pair.AccessToken)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

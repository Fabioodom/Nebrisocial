// Package auth implementa la integración OAuth2 con Google y GitHub para Nodal.
// Usa golang.org/x/oauth2 con flujo Authorization Code.
// Los Client IDs y Secrets se leen exclusivamente de variables de entorno.
package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"nodal/internal/platform/database"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

// ── Configuración OAuth2 ──────────────────────────────────────────────────────

// googleOAuthConfig construye la configuración OAuth2 para Google con los
// valores de entorno GOOGLE_CLIENT_ID y GOOGLE_CLIENT_SECRET.
func googleOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"), // e.g., http://localhost:8080/auth/google/callback
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

// githubOAuthConfig construye la configuración OAuth2 para GitHub con los
// valores de entorno GITHUB_CLIENT_ID y GITHUB_CLIENT_SECRET.
func githubOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GITHUB_REDIRECT_URL"), // e.g., http://localhost:8080/auth/github/callback
		Scopes:       []string{"user:email"},
		Endpoint:     github.Endpoint,
	}
}

// ── Generación de URLs de autorización ───────────────────────────────────────

// GetGoogleAuthURL genera la URL de redirección a Google para iniciar el flujo OAuth2.
// El parámetro state debe ser un valor aleatorio generado por GenerateOAuthState() para
// proteger contra ataques CSRF.
func GetGoogleAuthURL(state string) string {
	return googleOAuthConfig().AuthCodeURL(state, oauth2.AccessTypeOnline)
}

// GetGitHubAuthURL genera la URL de redirección a GitHub para iniciar el flujo OAuth2.
// El parámetro state debe ser un valor aleatorio generado por GenerateOAuthState() para
// proteger contra ataques CSRF.
func GetGitHubAuthURL(state string) string {
	return githubOAuthConfig().AuthCodeURL(state, oauth2.AccessTypeOnline)
}

// GenerateOAuthState genera un estado aleatorio y criptográficamente seguro para el
// parámetro state del flujo OAuth2 (protección CSRF).
// Debe almacenarse en una cookie o sesión para validarlo en el callback.
func GenerateOAuthState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("oauth: fallo al generar state: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// ── Callbacks de Google ───────────────────────────────────────────────────────

// googleUserInfo representa el subconjunto de datos que devuelve la API de Google.
type googleUserInfo struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// HandleGoogleCallback intercambia el código de autorización por tokens OAuth2,
// obtiene el email del usuario desde la API de Google y aplica un "upsert" en la BD:
//   - Si el email ya existe → devuelve el usuario existente.
//   - Si el email no existe → crea un nuevo usuario con password_hash vacío.
func HandleGoogleCallback(ctx context.Context, db *sql.DB, code string) (*database.User, error) {
	cfg := googleOAuthConfig()

	// Intercambiar código por token de acceso
	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oauth/google: fallo al intercambiar código: %w", err)
	}

	// Obtener información del usuario desde Google
	client := cfg.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, fmt.Errorf("oauth/google: fallo al obtener userinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth/google: userinfo respondió con status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("oauth/google: fallo al leer body: %w", err)
	}

	var userInfo googleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("oauth/google: fallo al parsear userinfo: %w", err)
	}

	if userInfo.Email == "" {
		return nil, fmt.Errorf("oauth/google: email vacío en la respuesta")
	}

	// Upsert en la BD
	return database.UpsertOAuthUser(db, sanitizeUsername(userInfo.Name, userInfo.Email), userInfo.Email)
}

// ── Callbacks de GitHub ───────────────────────────────────────────────────────

// githubUserInfo representa el subconjunto de datos que devuelve la API de GitHub.
type githubUserInfo struct {
	Login string `json:"login"`
	Email string `json:"email"`
}

// githubEmailInfo representa una entrada de la API de emails de GitHub.
type githubEmailInfo struct {
	Email   string `json:"email"`
	Primary bool   `json:"primary"`
}

// HandleGitHubCallback intercambia el código de autorización por tokens OAuth2,
// obtiene el email del usuario desde la API de GitHub y aplica un "upsert" en la BD.
// GitHub puede devolver el email nulo en el perfil público; en ese caso se consulta
// el endpoint /user/emails para obtener el email primario verificado.
func HandleGitHubCallback(ctx context.Context, db *sql.DB, code string) (*database.User, error) {
	cfg := githubOAuthConfig()

	// Intercambiar código por token de acceso
	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oauth/github: fallo al intercambiar código: %w", err)
	}

	client := cfg.Client(ctx, token)

	// Obtener perfil básico
	userInfo, err := fetchGitHubUser(client)
	if err != nil {
		return nil, err
	}

	// Si el email es público, usarlo directamente; de lo contrario buscar en /user/emails
	email := userInfo.Email
	if email == "" {
		email, err = fetchGitHubPrimaryEmail(client)
		if err != nil {
			return nil, err
		}
	}

	if email == "" {
		return nil, fmt.Errorf("oauth/github: no se pudo obtener email del usuario")
	}

	return database.UpsertOAuthUser(db, userInfo.Login, email)
}

// fetchGitHubUser obtiene el perfil básico del usuario autenticado en GitHub.
func fetchGitHubUser(client *http.Client) (*githubUserInfo, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth/github: fallo al obtener perfil: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth/github: /user respondió con status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("oauth/github: fallo al leer body de /user: %w", err)
	}

	var info githubUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("oauth/github: fallo al parsear /user: %w", err)
	}
	return &info, nil
}

// fetchGitHubPrimaryEmail consulta /user/emails y devuelve el email primario verificado.
func fetchGitHubPrimaryEmail(client *http.Client) (string, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("oauth/github: fallo al obtener emails: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oauth/github: /user/emails respondió con status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("oauth/github: fallo al leer body de /user/emails: %w", err)
	}

	var emails []githubEmailInfo
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", fmt.Errorf("oauth/github: fallo al parsear /user/emails: %w", err)
	}

	for _, e := range emails {
		if e.Primary {
			return e.Email, nil
		}
	}
	return "", nil
}

// sanitizeUsername genera un username limpio a partir del nombre completo o el email.
// Se usa cuando creamos un nuevo usuario OAuth y no tenemos un login explícito.
func sanitizeUsername(name, email string) string {
	if name != "" && len(name) <= 30 {
		// Eliminar espacios y truncar a 30 chars
		clean := ""
		for _, r := range name {
			if r != ' ' && r != '\t' {
				clean += string(r)
			}
			if len(clean) >= 30 {
				break
			}
		}
		if clean != "" {
			return clean
		}
	}
	// Fallback: usar la parte local del email
	for i, c := range email {
		if c == '@' {
			part := email[:i]
			if len(part) > 30 {
				return part[:30]
			}
			return part
		}
	}
	return email
}

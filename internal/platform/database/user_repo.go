// Package database contiene los repositorios de acceso a datos para Nodal.
// user_repo.go implementa las operaciones CRUD sobre la tabla 'users' usando database/sql puro.
package database

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"
)

// ErrUserNotFound se devuelve cuando no se encuentra un usuario en la BD.
var ErrUserNotFound = errors.New("user_repo: usuario no encontrado")

// ErrEmailAlreadyExists se devuelve cuando se intenta registrar un email ya existente.
var ErrEmailAlreadyExists = errors.New("user_repo: el email ya está registrado")

// User representa una fila de la tabla 'users'.
// Los campos corresponden 1:1 con las columnas del esquema SQL (migrations/init.sql + 03_oauth.sql).
type User struct {
	ID           string     // UUID (PK)
	Username     string
	Email        string
	PasswordHash *string    // Nullable: NULL para usuarios OAuth-only
	AvatarURL    *string    // Nullable
	Bio          *string    // Nullable
	IsPrivate    bool       // Privacidad del perfil
	AuthProvider string     // 'local' | 'google' | 'github'
	ProviderID   *string    // ID del usuario en el proveedor OAuth externo
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// CreateUser inserta un nuevo usuario local en la base de datos.
// Devuelve ErrEmailAlreadyExists si el email ya existe (violación de UNIQUE constraint).
// Nunca recibe la contraseña en texto plano; sólo el hash ya procesado.
func CreateUser(db *sql.DB, username, email, passwordHash string) (*User, error) {
	const query = `
		INSERT INTO users (username, email, password_hash, auth_provider)
		VALUES ($1, $2, $3, 'local')
		RETURNING id, username, email, password_hash, avatar_url, bio, is_private, auth_provider, provider_id, created_at, updated_at
	`

	user := &User{}
	err := db.QueryRow(query, username, email, passwordHash).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.AvatarURL,
		&user.Bio,
		&user.IsPrivate,
		&user.AuthProvider,
		&user.ProviderID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		// Detectar violación de UNIQUE (código PQ 23505) sin importar pq directamente
		if isUniqueViolation(err) {
			return nil, ErrEmailAlreadyExists
		}
		return nil, fmt.Errorf("user_repo: error al crear usuario: %w", err)
	}

	return user, nil
}

// FindUserByEmail busca un usuario por su dirección de email.
// Devuelve ErrUserNotFound si no existe ningún usuario con ese email.
func FindUserByEmail(db *sql.DB, email string) (*User, error) {
	const query = `
		SELECT id, username, email, password_hash, avatar_url, bio, is_private, auth_provider, provider_id, created_at, updated_at
		FROM users
		WHERE email = $1
		LIMIT 1
	`

	user := &User{}
	err := db.QueryRow(query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.AvatarURL,
		&user.Bio,
		&user.IsPrivate,
		&user.AuthProvider,
		&user.ProviderID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("user_repo: error al buscar usuario por email: %w", err)
	}

	return user, nil
}

// FindUserByID busca un usuario por su identificador UUID.
// Devuelve ErrUserNotFound si no existe ningún usuario con ese ID.
func FindUserByID(db *sql.DB, id string) (*User, error) {
	const query = `
		SELECT id, username, email, password_hash, avatar_url, bio, is_private, auth_provider, provider_id, created_at, updated_at
		FROM users
		WHERE id = $1
		LIMIT 1
	`

	user := &User{}
	err := db.QueryRow(query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.AvatarURL,
		&user.Bio,
		&user.IsPrivate,
		&user.AuthProvider,
		&user.ProviderID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("user_repo: error al buscar usuario por ID: %w", err)
	}

	return user, nil
}

// UpsertOAuthUser crea o actualiza un usuario OAuth en la BD.
// Usa INSERT ... ON CONFLICT para garantizar atomicidad:
//   - Si el provider+providerID ya existe → actualiza email/username y devuelve el usuario.
//   - Si el email ya existe con otro proveedor → devuelve el usuario existente sin modificar.
//   - Si no existe → inserta un nuevo usuario OAuth sin password_hash.
func UpsertOAuthUser(db *sql.DB, username, email string) (*User, error) {
	return CreateOrUpdateOAuthUser(db, "local", "", email, username)
}

// CreateOrUpdateOAuthUser es el método principal de upsert OAuth.
// provider: 'google' | 'github'
// providerID: sub de Google o login de GitHub
// email, username: datos del perfil del proveedor
func CreateOrUpdateOAuthUser(db *sql.DB, provider, providerID, email, username string) (*User, error) {
	// Sanitizar username para cumplir con VARCHAR(30)
	if len(username) > 30 {
		username = username[:30]
	}

	const query = `
		INSERT INTO users (username, email, auth_provider, provider_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (auth_provider, provider_id) WHERE provider_id IS NOT NULL
		DO UPDATE SET
			email      = EXCLUDED.email,
			updated_at = NOW()
		RETURNING id, username, email, password_hash, avatar_url, bio, is_private, auth_provider, provider_id, created_at, updated_at
	`

	user := &User{}
	err := db.QueryRow(query, username, email, provider, providerID).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.AvatarURL,
		&user.Bio,
		&user.IsPrivate,
		&user.AuthProvider,
		&user.ProviderID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		// Si hay conflicto de email (el usuario ya existe con otro proveedor),
		// buscarlo por email y devolverlo.
		if isUniqueViolation(err) {
			return FindUserByEmail(db, email)
		}
		return nil, fmt.Errorf("user_repo: error en upsert OAuth (%s): %w", provider, err)
	}
	return user, nil
}

// isUniqueViolation detecta si el error de PostgreSQL corresponde a una violación de UNIQUE.
// Evitamos importar github.com/lib/pq directamente aquí para mantener el repositorio desacoplado.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// El driver pq incluye el código PQ en el mensaje de error
	return containsCode(err.Error(), "23505") || contains(err.Error(), "unique constraint")
}

func containsCode(s, code string) bool {
	return len(s) >= len(code) && (s == code || len(s) > 0 && contains(s, code))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

// UpdateProfile actualiza la biografía y estado de privacidad de un usuario.
func UpdateProfile(db *sql.DB, userID string, bio string, isPrivate bool) error {
	const query = `
		UPDATE users
		SET bio = $2, is_private = $3, updated_at = NOW()
		WHERE id = $1
	`
	_, err := db.Exec(query, userID, bio, isPrivate)
	if err != nil {
		return fmt.Errorf("user_repo: failed to update profile: %w", err)
	}
	return nil
}

// ToggleFollow inserta o elimina la relación de seguimiento entre followerID y followingID.
// Retorna el nuevo estado de seguimiento: "none", "pending" o "accepted".
func ToggleFollow(db *sql.DB, followerID, followingID string) (string, error) {
	var currentStatus string
	err := db.QueryRow(`
		SELECT status FROM user_follows
		WHERE follower_id = $1 AND following_id = $2
	`, followerID, followingID).Scan(&currentStatus)
	
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "none", fmt.Errorf("user_repo: error checking follow: %w", err)
	}

	if err == nil {
		// Ya existe una relación (pending o accepted), entonces dejamos de seguir (unfollow)
		_, err = db.Exec(`
			DELETE FROM user_follows
			WHERE follower_id = $1 AND following_id = $2
		`, followerID, followingID)
		if err != nil {
			return currentStatus, fmt.Errorf("user_repo: error unfollowing user: %w", err)
		}
		
		// Eliminar notificaciones asociadas
		_, err = db.Exec(`
			DELETE FROM notifications
			WHERE user_id = $1 AND actor_id = $2 AND type = 'follow_request'
		`, followingID, followerID)
		if err != nil {
			log.Printf("WARN: failed to delete notifications: %v", err)
		}

		return "none", nil
	} else {
		// No existe relación, creamos una nueva. Primero consultamos la privacidad del usuario destino
		var isPrivate bool
		err = db.QueryRow(`
			SELECT is_private FROM users
			WHERE id = $1
		`, followingID).Scan(&isPrivate)
		if err != nil {
			return "none", fmt.Errorf("user_repo: error checking user privacy: %w", err)
		}

		var status string
		if isPrivate {
			status = "pending"
		} else {
			status = "accepted"
		}

		_, err = db.Exec(`
			INSERT INTO user_follows (follower_id, following_id, status)
			VALUES ($1, $2, $3)
		`, followerID, followingID, status)
		if err != nil {
			return "none", fmt.Errorf("user_repo: error following user: %w", err)
		}

		// Si es privado, enviamos una notificación
		if isPrivate {
			_, err = db.Exec(`
				INSERT INTO notifications (user_id, actor_id, type)
				VALUES ($1, $2, 'follow_request')
			`, followingID, followerID)
			if err != nil {
				log.Printf("WARN: failed to insert follow_request notification: %v", err)
			}
		}

		return status, nil
	}
}

// ProfileStats contiene estadísticas de seguidores/seguidos y relación.
type ProfileStats struct {
	FollowersCount int
	FollowingCount int
	IsFollowing    bool
	FollowStatus   string // 'none', 'pending', 'accepted'
}

// GetProfileStats devuelve la cantidad de seguidores, seguidos y si el usuario autenticado sigue al perfil.
func GetProfileStats(db *sql.DB, userID string, authenticatedUserID string) (*ProfileStats, error) {
	stats := &ProfileStats{
		FollowStatus: "none",
	}

	// 1. Count accepted followers
	err := db.QueryRow(`
		SELECT COUNT(*) FROM user_follows
		WHERE following_id = $1 AND status = 'accepted'
	`, userID).Scan(&stats.FollowersCount)
	if err != nil {
		return nil, fmt.Errorf("user_repo: error counting followers: %w", err)
	}

	// 2. Count accepted following
	err = db.QueryRow(`
		SELECT COUNT(*) FROM user_follows
		WHERE follower_id = $1 AND status = 'accepted'
	`, userID).Scan(&stats.FollowingCount)
	if err != nil {
		return nil, fmt.Errorf("user_repo: error counting following: %w", err)
	}

	// 3. Get follow status for authenticated user
	if authenticatedUserID != "" && authenticatedUserID != userID {
		var status string
		err = db.QueryRow(`
			SELECT status FROM user_follows
			WHERE follower_id = $1 AND following_id = $2
		`, authenticatedUserID, userID).Scan(&status)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				stats.FollowStatus = "none"
			} else {
				return nil, fmt.Errorf("user_repo: error getting follow status: %w", err)
			}
		} else {
			stats.FollowStatus = status
		}
	}

	stats.IsFollowing = stats.FollowStatus == "accepted"
	return stats, nil
}

// FindUserByUsername busca un usuario por su nombre de usuario (username).
// Devuelve ErrUserNotFound si no existe ningún usuario con ese username.
func FindUserByUsername(db *sql.DB, username string) (*User, error) {
	const query = `
		SELECT id, username, email, password_hash, avatar_url, bio, is_private, auth_provider, provider_id, created_at, updated_at
		FROM users
		WHERE username = $1
		LIMIT 1
	`

	user := &User{}
	err := db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.AvatarURL,
		&user.Bio,
		&user.IsPrivate,
		&user.AuthProvider,
		&user.ProviderID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("user_repo: error al buscar usuario por username: %w", err)
	}

	return user, nil
}

// Notification representa una notificación para el usuario.
type Notification struct {
	ID            string
	UserID        string
	ActorID       string
	ActorUsername string
	Type          string // 'follow_request', 'follow_accepted'
	ReferenceID   *string
	IsRead        bool
	CreatedAt     time.Time
}

// AcceptFollowRequest acepta una solicitud de seguimiento.
func AcceptFollowRequest(db *sql.DB, followerID, followingID string) error {
	// 1. Actualizar el estado de la relación a 'accepted'
	_, err := db.Exec(`
		UPDATE user_follows
		SET status = 'accepted'
		WHERE follower_id = $1 AND following_id = $2
	`, followerID, followingID)
	if err != nil {
		return fmt.Errorf("user_repo: error accepting follow request: %w", err)
	}

	// 2. Eliminar la notificación de solicitud de seguimiento pendiente
	_, err = db.Exec(`
		DELETE FROM notifications
		WHERE user_id = $1 AND actor_id = $2 AND type = 'follow_request'
	`, followingID, followerID)
	if err != nil {
		log.Printf("WARN: failed to delete follow_request notification: %v", err)
	}

	// 3. Crear una nueva notificación para el seguidor indicando que fue aceptado
	_, err = db.Exec(`
		INSERT INTO notifications (user_id, actor_id, type)
		VALUES ($1, $2, 'follow_accepted')
	`, followerID, followingID)
	if err != nil {
		log.Printf("WARN: failed to insert follow_accepted notification: %v", err)
	}

	return nil
}

// RejectFollowRequest rechaza/elimina una solicitud de seguimiento.
func RejectFollowRequest(db *sql.DB, followerID, followingID string) error {
	// 1. Eliminar la relación de seguimiento pendiente
	_, err := db.Exec(`
		DELETE FROM user_follows
		WHERE follower_id = $1 AND following_id = $2
	`, followerID, followingID)
	if err != nil {
		return fmt.Errorf("user_repo: error rejecting follow request: %w", err)
	}

	// 2. Eliminar la notificación de solicitud pendiente
	_, err = db.Exec(`
		DELETE FROM notifications
		WHERE user_id = $1 AND actor_id = $2 AND type = 'follow_request'
	`, followingID, followerID)
	if err != nil {
		log.Printf("WARN: failed to delete notifications on reject: %v", err)
	}

	return nil
}

// ListNotifications obtiene todas las notificaciones de un usuario.
func ListNotifications(db *sql.DB, userID string) ([]Notification, error) {
	const query = `
		SELECT n.id, n.user_id, n.actor_id, u.username, n.type, n.reference_id, n.is_read, n.created_at
		FROM notifications n
		JOIN users u ON n.actor_id = u.id
		WHERE n.user_id = $1
		ORDER BY n.created_at DESC
	`
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("user_repo: error querying notifications: %w", err)
	}
	defer rows.Close()

	var list []Notification
	for rows.Next() {
		var n Notification
		err = rows.Scan(
			&n.ID,
			&n.UserID,
			&n.ActorID,
			&n.ActorUsername,
			&n.Type,
			&n.ReferenceID,
			&n.IsRead,
			&n.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("user_repo: error scanning notification: %w", err)
		}
		list = append(list, n)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("user_repo: error iterating notifications rows: %w", err)
	}

	return list, nil
}

// MarkNotificationsAsRead marca todas las notificaciones del usuario como leídas.
func MarkNotificationsAsRead(db *sql.DB, userID string) error {
	_, err := db.Exec(`
		UPDATE notifications
		SET is_read = true
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("user_repo: error marking notifications as read: %w", err)
	}
	return nil
}

// GetPendingFollowRequests obtiene las notificaciones de tipo follow_request no leídas junto con los datos del usuario que la envía.
func GetPendingFollowRequests(db *sql.DB, userID string) ([]Notification, error) {
	const query = `
		SELECT n.id, n.user_id, n.actor_id, u.username, n.type, n.reference_id, n.is_read, n.created_at
		FROM notifications n
		JOIN users u ON n.actor_id = u.id
		WHERE n.user_id = $1 AND n.type = 'follow_request' AND n.is_read = false
		ORDER BY n.created_at DESC
	`
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("user_repo: error querying pending follow requests: %w", err)
	}
	defer rows.Close()

	var list []Notification
	for rows.Next() {
		var n Notification
		err = rows.Scan(
			&n.ID,
			&n.UserID,
			&n.ActorID,
			&n.ActorUsername,
			&n.Type,
			&n.ReferenceID,
			&n.IsRead,
			&n.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("user_repo: error scanning follow request notification: %w", err)
		}
		list = append(list, n)
	}

	return list, rows.Err()
}

// AcceptFollowRequestByID acepta una solicitud de seguimiento a partir del ID de la notificación y del ID del usuario autenticado (following_id).
func AcceptFollowRequestByID(db *sql.DB, notificationID, userID string) error {
	var followerID string
	err := db.QueryRow(`
		SELECT actor_id 
		FROM notifications 
		WHERE id = $1 AND user_id = $2 AND type = 'follow_request'
	`, notificationID, userID).Scan(&followerID)
	if err != nil {
		return fmt.Errorf("user_repo: failed to find follow request notification: %w", err)
	}

	return AcceptFollowRequest(db, followerID, userID)
}

// RejectFollowRequestByID rechaza una solicitud de seguimiento a partir del ID de la notificación y del ID del usuario autenticado (following_id).
func RejectFollowRequestByID(db *sql.DB, notificationID, userID string) error {
	var followerID string
	err := db.QueryRow(`
		SELECT actor_id 
		FROM notifications 
		WHERE id = $1 AND user_id = $2 AND type = 'follow_request'
	`, notificationID, userID).Scan(&followerID)
	if err != nil {
		return fmt.Errorf("user_repo: failed to find follow request notification: %w", err)
	}

	return RejectFollowRequest(db, followerID, userID)
}

// SearchUsers busca usuarios cuyo nombre de usuario o biografía coincida con la consulta (ignorando espacios y distinguiendo mayúsculas/minúsculas).
func SearchUsers(db *sql.DB, query string) ([]User, error) {
	const sqlQuery = `
		SELECT id, username, COALESCE(avatar_url, ''), COALESCE(bio, '')
		FROM users
		WHERE REPLACE(username, ' ', '') ILIKE '%' || REPLACE($1, ' ', '') || '%'
		   OR REPLACE(COALESCE(bio, ''), ' ', '') ILIKE '%' || REPLACE($1, ' ', '') || '%'
		ORDER BY username ASC
	`

	rows, err := db.Query(sqlQuery, query)
	if err != nil {
		return nil, fmt.Errorf("user_repo: error al buscar usuarios: %w", err)
	}
	defer rows.Close()

	var list []User
	for rows.Next() {
		var u User
		var avatarURL, bio string
		err = rows.Scan(
			&u.ID,
			&u.Username,
			&avatarURL,
			&bio,
		)
		if err != nil {
			return nil, fmt.Errorf("user_repo: error al escanear usuario: %w", err)
		}
		u.AvatarURL = &avatarURL
		u.Bio = &bio
		list = append(list, u)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("user_repo: error iterando filas de usuarios: %w", err)
	}

	return list, nil
}


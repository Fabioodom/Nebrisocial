// Package database contiene los repositorios de acceso a datos para Nodal.
// user_repo.go implementa las operaciones CRUD sobre la tabla 'users' usando database/sql puro.
package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrUserNotFound se devuelve cuando no se encuentra un usuario en la BD.
var ErrUserNotFound = errors.New("user_repo: usuario no encontrado")

// ErrEmailAlreadyExists se devuelve cuando se intenta registrar un email ya existente.
var ErrEmailAlreadyExists = errors.New("user_repo: el email ya está registrado")

// User representa una fila de la tabla 'users'.
// Los campos corresponden 1:1 con las columnas del esquema SQL (migrations/init.sql).
type User struct {
	ID           string     // UUID (PK)
	Username     string
	Email        string
	PasswordHash string
	AvatarURL    *string    // Nullable
	Bio          *string    // Nullable
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// CreateUser inserta un nuevo usuario en la base de datos.
// Devuelve ErrEmailAlreadyExists si el email ya existe (violación de UNIQUE constraint).
// Nunca recibe la contraseña en texto plano; sólo el hash ya procesado.
func CreateUser(db *sql.DB, username, email, passwordHash string) (*User, error) {
	const query = `
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, username, email, password_hash, avatar_url, bio, created_at, updated_at
	`

	user := &User{}
	err := db.QueryRow(query, username, email, passwordHash).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.AvatarURL,
		&user.Bio,
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
		SELECT id, username, email, password_hash, avatar_url, bio, created_at, updated_at
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
		SELECT id, username, email, password_hash, avatar_url, bio, created_at, updated_at
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

// UpsertOAuthUser crea un usuario si su email no existe, o lo devuelve si ya existe.
// Se utiliza en los flujos OAuth2 (Google, GitHub) donde no hay contraseña local.
// passwordHash se pasa como cadena vacía para usuarios OAuth-only.
func UpsertOAuthUser(db *sql.DB, username, email string) (*User, error) {
	// Intentar encontrar primero
	existing, err := FindUserByEmail(db, email)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return nil, fmt.Errorf("user_repo: error en upsert OAuth: %w", err)
	}

	// No existe → crear con password_hash vacío
	return CreateUser(db, username, email, "")
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

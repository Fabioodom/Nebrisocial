package database

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	_ "github.com/lib/pq" // PostgreSQL driver
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Node represents the Node entity in the database.
type Node struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// GenerateSlug crea un slug URL-friendly a partir de un título.
// Ejemplo: "Mi Primer Nodo!" -> "mi-primer-nodo"
func GenerateSlug(title string) string {
	// Normalizar unicode: descomponer caracteres acentuados (é -> e + ́)
	t := transform.Chain(norm.NFD, transform.RemoveFunc(func(r rune) bool {
		return unicode.Is(unicode.Mn, r) // Mn = marcas no espaciadoras (tildes, etc.)
	}), norm.NFC)
	result, _, _ := transform.String(t, title)

	// Pasar a minúsculas
	result = strings.ToLower(result)

	// Reemplazar cualquier carácter no alfanumérico por guion
	re := regexp.MustCompile(`[^a-z0-9]+`)
	result = re.ReplaceAllString(result, "-")

	// Eliminar guiones al inicio y al final
	result = strings.Trim(result, "-")

	return result
}

// CreateNode inserts a new node into the database with slug, title and description.
func CreateNode(db *sql.DB, title, description string) error {
	slug := GenerateSlug(title)
	query := `INSERT INTO nodes (slug, title, description) VALUES ($1, $2, $3)`
	_, err := db.Exec(query, slug, title, description)
	if err != nil {
		return fmt.Errorf("failed to create node: %w", err)
	}
	return nil
}

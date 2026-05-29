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
	Category    string    `json:"category"`
	ImageURL    *string   `json:"image_url"`
	CreatedAt   time.Time `json:"created_at"`
}

// ChatMessage represents a single chat message in the database.
type ChatMessage struct {
	ID        string    `json:"id"`
	NodeID    string    `json:"node_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
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

// CreateNode inserts a new node into the database with slug, title, description, category and image_url.
func CreateNode(db *sql.DB, title, description, category string, imageURL *string) (string, error) {
	slug := GenerateSlug(title)
	// Añadimos RETURNING id para que PostgreSQL nos devuelva el UUID generado
	query := `INSERT INTO nodes (slug, title, description, category, image_url) VALUES ($1, $2, $3, $4, $5) RETURNING id`
	
	var id string
	err := db.QueryRow(query, slug, title, description, category, imageURL).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to create node: %w", err)
	}
	return id, nil
}

// ListNodes retrieves all nodes ordered by creation date descending.
func ListNodes(db *sql.DB) ([]Node, error) {
	query := `SELECT id, COALESCE(slug,''), title, COALESCE(description,''), COALESCE(category,''), image_url, created_at
	          FROM nodes
	          ORDER BY created_at DESC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.Slug, &n.Title, &n.Description, &n.Category, &n.ImageURL, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// GetNodeByID retrieves a single node by its UUID.
func GetNodeByID(db *sql.DB, id string) (*Node, error) {
	query := `SELECT id, COALESCE(slug,''), title, COALESCE(description,''), COALESCE(category,''), image_url, created_at
	          FROM nodes WHERE id = $1`

	var n Node
	err := db.QueryRow(query, id).Scan(&n.ID, &n.Slug, &n.Title, &n.Description, &n.Category, &n.ImageURL, &n.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("node not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	return &n, nil
}

// ListChatMessages retrieves the last N messages for a given node, oldest first.
func ListChatMessages(db *sql.DB, nodeID string, limit int) ([]ChatMessage, error) {
	query := `SELECT id, node_id, content, created_at
	          FROM chat_messages
	          WHERE node_id = $1
	          ORDER BY created_at ASC
	          LIMIT $2`

	rows, err := db.Query(query, nodeID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list chat messages: %w", err)
	}
	defer rows.Close()

	var messages []ChatMessage
	for rows.Next() {
		var m ChatMessage
		if err := rows.Scan(&m.ID, &m.NodeID, &m.Content, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan chat message: %w", err)
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// CreateChatMessage inserts a new message in the chat of a node.
// user_id is nullable in the schema, so we omit it for anonymous users.
func CreateChatMessage(db *sql.DB, nodeID, content string) (*ChatMessage, error) {
	query := `INSERT INTO chat_messages (node_id, content) VALUES ($1, $2)
	          RETURNING id, node_id, content, created_at`

	var m ChatMessage
	err := db.QueryRow(query, nodeID, content).Scan(&m.ID, &m.NodeID, &m.Content, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat message: %w", err)
	}
	return &m, nil
}

// Thread represents an AI-generated summary thread for a node.
type Thread struct {
	ID            string    `json:"id"`
	NodeID        string    `json:"node_id"`
	Title         string    `json:"title"`
	Body          string    `json:"body"`
	IsAIGenerated bool      `json:"is_ai_generated"`
	Pinned        bool      `json:"pinned"`
	CreatedAt     time.Time `json:"created_at"`
}

// ListAIThreadsByNodeID retrieves all AI-generated threads for a node, oldest first.
// Only returns threads where is_ai_generated = true.
func ListAIThreadsByNodeID(db *sql.DB, nodeID string) ([]Thread, error) {
	query := `SELECT id, node_id, COALESCE(title,''), body, is_ai_generated, pinned, created_at
	          FROM threads
	          WHERE node_id = $1 AND is_ai_generated = true
	          ORDER BY created_at ASC`

	rows, err := db.Query(query, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to list AI threads: %w", err)
	}
	defer rows.Close()

	var threads []Thread
	for rows.Next() {
		var t Thread
		if err := rows.Scan(&t.ID, &t.NodeID, &t.Title, &t.Body, &t.IsAIGenerated, &t.Pinned, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan thread: %w", err)
		}
		threads = append(threads, t)
	}
	return threads, rows.Err()
}

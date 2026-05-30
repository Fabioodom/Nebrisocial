package database

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
	"unicode"

	_ "github.com/lib/pq" // PostgreSQL driver
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// CreateNotification inserta una nueva notificación en la base de datos.
// referenceID puede ser vacío si no aplica.
func CreateNotification(db *sql.DB, userID, actorID, notifType, referenceID string) error {
	var refID interface{}
	if referenceID != "" {
		refID = referenceID
	}
	_, err := db.Exec(`
		INSERT INTO notifications (user_id, actor_id, type, reference_id)
		VALUES ($1, $2, $3, $4)
	`, userID, actorID, notifType, refID)
	if err != nil {
		return fmt.Errorf("CreateNotification: %w", err)
	}
	return nil
}

// Node represents the Node entity in the database.
type Node struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	ImageURL    *string   `json:"image_url"`
	Username    string    `json:"username"`
	UserInitial string    `json:"user_initial"`
	CreatedAt   time.Time `json:"created_at"`
	LikesCount  int       `json:"likes_count"`
	IsLiked     bool      `json:"is_liked"`
	IsSaved     bool      `json:"is_saved"`
}

// Trend represents a trending hashtag with its occurrence count.
type Trend struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// ChatMessage represents a single chat message in the database.
type ChatMessage struct {
	ID             string        `json:"id"`
	NodeID         string        `json:"node_id"`
	Content        string        `json:"content"`
	UserID         *string       `json:"user_id"`
	Username       string        `json:"username"`
	UserInitial    string        `json:"user_initial"`
	CreatedAt      time.Time     `json:"created_at"`
	ParentID       *string       `json:"parent_id,omitempty"`
	ParentUsername *string       `json:"parent_username,omitempty"`
	ParentContent  *string       `json:"parent_content,omitempty"`
	Replies        []ChatMessage `json:"replies,omitempty"`
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

// CreateNode inserts a new node into the database with slug, title, description, category, image_url and owner_id.
func CreateNode(db *sql.DB, title, description, category string, imageURL *string, ownerID *string) (string, error) {
	slug := GenerateSlug(title)
	// Añadimos RETURNING id para que PostgreSQL nos devuelva el UUID generado
	query := `INSERT INTO nodes (slug, title, description, category, image_url, owner_id) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	
	var id string
	err := db.QueryRow(query, slug, title, description, category, imageURL, ownerID).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to create node: %w", err)
	}
	return id, nil
}

// ListNodes retrieves all nodes ordered by creation date descending.
func ListNodes(db *sql.DB, userID *string) ([]Node, error) {
	var userUUID sql.NullString
	if userID != nil {
		userUUID.String = *userID
		userUUID.Valid = true
	}

	query := `SELECT n.id, COALESCE(n.slug,''), n.title, COALESCE(n.description,''), COALESCE(n.category,''), n.image_url, COALESCE(u.username, 'Desconocido'), n.created_at,
	                 (SELECT COUNT(*) FROM node_likes WHERE node_id = n.id) AS likes_count,
	                 CASE WHEN $1::uuid IS NOT NULL AND EXISTS (SELECT 1 FROM node_likes WHERE node_id = n.id AND user_id = $1::uuid) THEN TRUE ELSE FALSE END AS is_liked,
	                 CASE WHEN $1::uuid IS NOT NULL AND EXISTS (SELECT 1 FROM saved_nodes WHERE node_id = n.id AND user_id = $1::uuid) THEN TRUE ELSE FALSE END AS is_saved
	          FROM nodes n
	          LEFT JOIN users u ON n.owner_id = u.id
	          ORDER BY n.created_at DESC`

	rows, err := db.Query(query, userUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.Slug, &n.Title, &n.Description, &n.Category, &n.ImageURL, &n.Username, &n.CreatedAt, &n.LikesCount, &n.IsLiked, &n.IsSaved); err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}
		if n.Username != "" {
			n.UserInitial = strings.ToUpper(string([]rune(n.Username)[0]))
		} else {
			n.UserInitial = "A"
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// GetNodeByID retrieves a single node by its UUID.
func GetNodeByID(db *sql.DB, id string, userID *string) (*Node, error) {
	var userUUID sql.NullString
	if userID != nil {
		userUUID.String = *userID
		userUUID.Valid = true
	}

	query := `SELECT n.id, COALESCE(n.slug,''), n.title, COALESCE(n.description,''), COALESCE(n.category,''), n.image_url, COALESCE(u.username, 'Desconocido'), n.created_at,
	                 (SELECT COUNT(*) FROM node_likes WHERE node_id = n.id) AS likes_count,
	                 CASE WHEN $2::uuid IS NOT NULL AND EXISTS (SELECT 1 FROM node_likes WHERE node_id = n.id AND user_id = $2::uuid) THEN TRUE ELSE FALSE END AS is_liked,
	                 CASE WHEN $2::uuid IS NOT NULL AND EXISTS (SELECT 1 FROM saved_nodes WHERE node_id = n.id AND user_id = $2::uuid) THEN TRUE ELSE FALSE END AS is_saved
	          FROM nodes n
	          LEFT JOIN users u ON n.owner_id = u.id
	          WHERE n.id = $1`

	var n Node
	err := db.QueryRow(query, id, userUUID).Scan(&n.ID, &n.Slug, &n.Title, &n.Description, &n.Category, &n.ImageURL, &n.Username, &n.CreatedAt, &n.LikesCount, &n.IsLiked, &n.IsSaved)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("node not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	if n.Username != "" {
		n.UserInitial = strings.ToUpper(string([]rune(n.Username)[0]))
	} else {
		n.UserInitial = "A"
	}
	return &n, nil
}


// ListChatMessages retrieves the last N messages for a given node, oldest first, and nests replies inside parent messages.
func ListChatMessages(db *sql.DB, nodeID string, limit int) ([]ChatMessage, error) {
	query := `SELECT c.id, c.node_id, c.content, c.user_id, COALESCE(u.username, 'Anónimo'), c.created_at,
	                 c.parent_id, p.content AS parent_content, COALESCE(pu.username, 'Anónimo') AS parent_username
	          FROM chat_messages c
	          LEFT JOIN users u ON c.user_id = u.id
	          LEFT JOIN chat_messages p ON c.parent_id = p.id
	          LEFT JOIN users pu ON p.user_id = pu.id
	          WHERE c.node_id = $1
	          ORDER BY c.created_at ASC
	          LIMIT $2`

	rows, err := db.Query(query, nodeID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list chat messages: %w", err)
	}
	defer rows.Close()

	var allMessages []ChatMessage
	for rows.Next() {
		var m ChatMessage
		var parentID, parentContent, parentUsername sql.NullString
		if err := rows.Scan(&m.ID, &m.NodeID, &m.Content, &m.UserID, &m.Username, &m.CreatedAt, &parentID, &parentContent, &parentUsername); err != nil {
			return nil, fmt.Errorf("failed to scan chat message: %w", err)
		}
		if parentID.Valid {
			m.ParentID = &parentID.String
		}
		if parentContent.Valid {
			m.ParentContent = &parentContent.String
		}
		if parentUsername.Valid {
			m.ParentUsername = &parentUsername.String
		}
		if m.Username != "" {
			m.UserInitial = strings.ToUpper(string([]rune(m.Username)[0]))
		} else {
			m.UserInitial = "A"
		}
		m.Replies = []ChatMessage{}
		allMessages = append(allMessages, m)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	// Agrupación en 1 nivel de anidamiento (estilo Reddit simple)
	// Usamos un mapa de ID -> *ChatMessage para construir las relaciones.
	// Para evitar problemas de reasignación de memoria de slices,
	// primero copiamos todos los mensajes a un mapa con copias persistentes en memoria (punteros directos).
	msgMap := make(map[string]*ChatMessage)
	for i := range allMessages {
		msgCopy := allMessages[i]
		msgMap[allMessages[i].ID] = &msgCopy
	}

	var roots []*ChatMessage
	// Identificamos raíces y asignamos hijos
	for i := range allMessages {
		m := msgMap[allMessages[i].ID]
		pID := m.ParentID
		if pID == nil || *pID == "" {
			// Es raíz
			roots = append(roots, m)
		} else {
			// Es hijo. Intentamos buscar a su padre en el mapa.
			if parentNode, ok := msgMap[*pID]; ok {
				parentNode.Replies = append(parentNode.Replies, *m)
			} else {
				// Si no encuentra el padre (ej: fuera del límite/paginación), lo tratamos como raíz
				roots = append(roots, m)
			}
		}
	}

	// Convertimos a slice de valores []ChatMessage para cumplir con la firma
	result := make([]ChatMessage, len(roots))
	for i, r := range roots {
		result[i] = *r
	}

	return result, nil
}

// CreateChatMessage inserts a new message in the chat of a node.
func CreateChatMessage(db *sql.DB, nodeID, content string, userID, parentID *string) (*ChatMessage, error) {
	// Si parentID es puntero a string vacío, lo cambiamos a nil para que se guarde como NULL en PostgreSQL
	if parentID != nil && *parentID == "" {
		parentID = nil
	}

	query := `INSERT INTO chat_messages (node_id, content, user_id, parent_id) VALUES ($1, $2, $3, $4)
	          RETURNING id, node_id, content, user_id, parent_id, created_at`

	var m ChatMessage
	err := db.QueryRow(query, nodeID, content, userID, parentID).Scan(&m.ID, &m.NodeID, &m.Content, &m.UserID, &m.ParentID, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat message: %w", err)
	}
	
	m.Username = "Anónimo"
	if userID != nil {
		var username string
		err := db.QueryRow(`SELECT username FROM users WHERE id = $1`, *userID).Scan(&username)
		if err == nil {
			m.Username = username
		}
	}
	if m.Username != "" {
		m.UserInitial = strings.ToUpper(string([]rune(m.Username)[0]))
	} else {
		m.UserInitial = "A"
	}

	// Populate parent details and fire reply_received notification if this is a reply
	if parentID != nil && *parentID != "" {
		var parentContent, parentUsername string
		var parentUserID sql.NullString
		err := db.QueryRow(`
			SELECT c.content, COALESCE(u.username, 'Anónimo'), c.user_id
			FROM chat_messages c
			LEFT JOIN users u ON c.user_id = u.id
			WHERE c.id = $1
		`, *parentID).Scan(&parentContent, &parentUsername, &parentUserID)
		if err == nil {
			m.ParentContent = &parentContent
			m.ParentUsername = &parentUsername

			// Notificar al autor del mensaje padre si es distinto al que responde
			if parentUserID.Valid && parentUserID.String != "" && userID != nil && parentUserID.String != *userID {
				if nErr := CreateNotification(db, parentUserID.String, *userID, "reply_received", nodeID); nErr != nil {
					log.Printf("WARN: failed to create reply_received notification: %v", nErr)
				}
			}
		}
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

// ToggleLike inserts or deletes a user like for a node. Returns true if liked, false if unliked.
// When liking, fires a 'node_liked' notification to the node owner (unless the liker IS the owner).
func ToggleLike(db *sql.DB, userID, nodeID string) (bool, error) {
	tx, err := db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var exists bool
	err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM node_likes WHERE user_id = $1 AND node_id = $2)`, userID, nodeID).Scan(&exists)
	if err != nil {
		return false, err
	}

	if exists {
		_, err = tx.Exec(`DELETE FROM node_likes WHERE user_id = $1 AND node_id = $2`, userID, nodeID)
		if err != nil {
			return false, err
		}
		err = tx.Commit()
		return false, err
	} else {
		_, err = tx.Exec(`INSERT INTO node_likes (user_id, node_id) VALUES ($1, $2)`, userID, nodeID)
		if err != nil {
			return false, err
		}
		err = tx.Commit()
		if err != nil {
			return true, err
		}

		// Disparar notificación al dueño del nodo (si no es el mismo usuario)
		var ownerID string
		qErr := db.QueryRow(`SELECT COALESCE(owner_id::text, '') FROM nodes WHERE id = $1`, nodeID).Scan(&ownerID)
		if qErr == nil && ownerID != "" && ownerID != userID {
			if nErr := CreateNotification(db, ownerID, userID, "node_liked", nodeID); nErr != nil {
				log.Printf("WARN: failed to create node_liked notification: %v", nErr)
			}
		}
		return true, nil
	}
}

// ToggleSave inserts or deletes a user save for a node. Returns true if saved, false if unsaved.
func ToggleSave(db *sql.DB, userID, nodeID string) (bool, error) {
	tx, err := db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var exists bool
	err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM saved_nodes WHERE user_id = $1 AND node_id = $2)`, userID, nodeID).Scan(&exists)
	if err != nil {
		return false, err
	}

	if exists {
		_, err = tx.Exec(`DELETE FROM saved_nodes WHERE user_id = $1 AND node_id = $2`, userID, nodeID)
		if err != nil {
			return false, err
		}
		err = tx.Commit()
		return false, err
	} else {
		_, err = tx.Exec(`INSERT INTO saved_nodes (user_id, node_id) VALUES ($1, $2)`, userID, nodeID)
		if err != nil {
			return false, err
		}
		err = tx.Commit()
		return true, err
	}
}

// ToggleSaveNode inserts or deletes a user save for a node and returns the new saved status.
func ToggleSaveNode(db *sql.DB, userID string, nodeID string) (bool, error) {
	return ToggleSave(db, userID, nodeID)
}

// ListNodesByOwner retrieves all nodes owned by a user, ordered by creation date descending.
func ListNodesByOwner(db *sql.DB, ownerID string) ([]Node, error) {
	query := `SELECT n.id, COALESCE(n.slug,''), n.title, COALESCE(n.description,''), COALESCE(n.category,''), n.image_url, COALESCE(u.username, 'Desconocido'), n.created_at,
	                 (SELECT COUNT(*) FROM node_likes WHERE node_id = n.id) AS likes_count,
	                 EXISTS (SELECT 1 FROM node_likes WHERE node_id = n.id AND user_id = $1) AS is_liked,
	                 EXISTS (SELECT 1 FROM saved_nodes WHERE node_id = n.id AND user_id = $1) AS is_saved
	          FROM nodes n
	          LEFT JOIN users u ON n.owner_id = u.id
	          WHERE n.owner_id = $1
	          ORDER BY n.created_at DESC`

	rows, err := db.Query(query, ownerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes by owner: %w", err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.Slug, &n.Title, &n.Description, &n.Category, &n.ImageURL, &n.Username, &n.CreatedAt, &n.LikesCount, &n.IsLiked, &n.IsSaved); err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}
		if n.Username != "" {
			n.UserInitial = strings.ToUpper(string([]rune(n.Username)[0]))
		} else {
			n.UserInitial = "A"
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// ListSavedNodes retrieves all nodes saved by a user, ordered by creation date descending.
func ListSavedNodes(db *sql.DB, userID string) ([]Node, error) {
	query := `SELECT n.id, COALESCE(n.slug,''), n.title, COALESCE(n.description,''), COALESCE(n.category,''), n.image_url, COALESCE(u.username, 'Desconocido'), n.created_at,
	                 (SELECT COUNT(*) FROM node_likes WHERE node_id = n.id) AS likes_count,
	                 EXISTS (SELECT 1 FROM node_likes WHERE node_id = n.id AND user_id = $1) AS is_liked,
	                 TRUE AS is_saved
	          FROM saved_nodes s
	          JOIN nodes n ON s.node_id = n.id
	          LEFT JOIN users u ON n.owner_id = u.id
	          WHERE s.user_id = $1
	          ORDER BY n.created_at DESC`

	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list saved nodes: %w", err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.Slug, &n.Title, &n.Description, &n.Category, &n.ImageURL, &n.Username, &n.CreatedAt, &n.LikesCount, &n.IsLiked, &n.IsSaved); err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}
		if n.Username != "" {
			n.UserInitial = strings.ToUpper(string([]rune(n.Username)[0]))
		} else {
			n.UserInitial = "A"
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// SearchNodes searches nodes where title or description matches the query string (case-insensitive).
func SearchNodes(db *sql.DB, queryStr string, userID *string) ([]Node, error) {
	var userUUID sql.NullString
	if userID != nil {
		userUUID.String = *userID
		userUUID.Valid = true
	}

	query := `SELECT n.id, COALESCE(n.slug,''), n.title, COALESCE(n.description,''), COALESCE(n.category,''), n.image_url, COALESCE(u.username, 'Desconocido'), n.created_at,
	                 (SELECT COUNT(*) FROM node_likes WHERE node_id = n.id) AS likes_count,
	                 CASE WHEN $2::uuid IS NOT NULL AND EXISTS (SELECT 1 FROM node_likes WHERE node_id = n.id AND user_id = $2::uuid) THEN TRUE ELSE FALSE END AS is_liked,
	                 CASE WHEN $2::uuid IS NOT NULL AND EXISTS (SELECT 1 FROM saved_nodes WHERE node_id = n.id AND user_id = $2::uuid) THEN TRUE ELSE FALSE END AS is_saved
	          FROM nodes n
	          LEFT JOIN users u ON n.owner_id = u.id
	          WHERE REPLACE(n.title, ' ', '') ILIKE '%' || REPLACE($1, ' ', '') || '%'
	             OR REPLACE(n.description, ' ', '') ILIKE '%' || REPLACE($1, ' ', '') || '%'
	             OR REPLACE(n.category, ' ', '') ILIKE '%' || REPLACE($1, ' ', '') || '%'
	          ORDER BY n.created_at DESC`


	rows, err := db.Query(query, queryStr, userUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to search nodes: %w", err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.Slug, &n.Title, &n.Description, &n.Category, &n.ImageURL, &n.Username, &n.CreatedAt, &n.LikesCount, &n.IsLiked, &n.IsSaved); err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}
		if n.Username != "" {
			n.UserInitial = strings.ToUpper(string([]rune(n.Username)[0]))
		} else {
			n.UserInitial = "A"
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// GetPopularNodes retrieves the top limit nodes ordered by likes count descending.
func GetPopularNodes(db *sql.DB, limit int) ([]Node, error) {
	query := `SELECT n.id, COALESCE(n.slug,''), n.title, COALESCE(n.description,''), COALESCE(n.category,''), n.image_url, COALESCE(u.username, 'Desconocido'), n.created_at,
	                 (SELECT COUNT(*) FROM node_likes WHERE node_id = n.id) AS likes_count
	          FROM nodes n
	          LEFT JOIN users u ON n.owner_id = u.id
	          ORDER BY likes_count DESC, n.created_at DESC
	          LIMIT $1`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get popular nodes: %w", err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.Slug, &n.Title, &n.Description, &n.Category, &n.ImageURL, &n.Username, &n.CreatedAt, &n.LikesCount); err != nil {
			return nil, fmt.Errorf("failed to scan popular node: %w", err)
		}
		if n.Username != "" {
			n.UserInitial = strings.ToUpper(string([]rune(n.Username)[0]))
		} else {
			n.UserInitial = "A"
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// GetCategoriesWithNodes retrieves unique active categories and their nodes.
func GetCategoriesWithNodes(db *sql.DB) (map[string][]Node, error) {
	query := `SELECT n.id, COALESCE(n.slug,''), n.title, COALESCE(n.description,''), COALESCE(n.category,''), n.image_url, COALESCE(u.username, 'Desconocido'), n.created_at
	          FROM nodes n
	          LEFT JOIN users u ON n.owner_id = u.id
	          WHERE n.category IS NOT NULL AND n.category <> ''
	          ORDER BY n.category ASC, n.created_at DESC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories with nodes: %w", err)
	}
	defer rows.Close()

	categoriesMap := make(map[string][]Node)
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.Slug, &n.Title, &n.Description, &n.Category, &n.ImageURL, &n.Username, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan node for categories: %w", err)
		}
		if n.Username != "" {
			n.UserInitial = strings.ToUpper(string([]rune(n.Username)[0]))
		} else {
			n.UserInitial = "A"
		}
		catKey := strings.ToUpper(n.Category)
		categoriesMap[catKey] = append(categoriesMap[catKey], n)
	}
	return categoriesMap, rows.Err()
}

// TrendingTopic represents a trending topic shown on the right sidebar.
type TrendingTopic struct {
	Category string `json:"category"`
	Emoji    string `json:"emoji"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
}

// GetTrendingTopics queries the database to extract trending categories based on recent node creation and chat message activity,
// and maps them dynamically to rich TrendingTopic metadata.
func GetTrendingTopics(db *sql.DB, limit int) ([]TrendingTopic, error) {
	query := `
		WITH node_activity AS (
			SELECT category, COUNT(*) * 5 AS score
			FROM nodes
			WHERE created_at >= NOW() - INTERVAL '48 hours' AND category IS NOT NULL AND category <> ''
			GROUP BY category
		),
		chat_activity AS (
			SELECT n.category, COUNT(c.id) * 1 AS score
			FROM chat_messages c
			JOIN nodes n ON c.node_id = n.id
			WHERE c.created_at >= NOW() - INTERVAL '48 hours' AND n.category IS NOT NULL AND n.category <> ''
			GROUP BY n.category
		),
		all_recent AS (
			SELECT category, score FROM node_activity
			UNION ALL
			SELECT category, score FROM chat_activity
		),
		recent_scores AS (
			SELECT category, SUM(score) AS total_score
			FROM all_recent
			GROUP BY category
		),
		fallback_scores AS (
			SELECT category, COUNT(*) AS total_score
			FROM nodes
			WHERE category IS NOT NULL AND category <> ''
			GROUP BY category
		)
		SELECT f.category, COALESCE(r.total_score, 0) + COALESCE(f.total_score, 0) * 0.1 AS final_score
		FROM fallback_scores f
		LEFT JOIN recent_scores r ON f.category = r.category
		ORDER BY final_score DESC
		LIMIT $1
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get trending topics: %w", err)
	}
	defer rows.Close()

	var topics []TrendingTopic
	for rows.Next() {
		var category string
		var finalScore float64
		if err := rows.Scan(&category, &finalScore); err != nil {
			return nil, fmt.Errorf("failed to scan trending topic: %w", err)
		}
		count := int(finalScore)
		if count <= 0 {
			count = 1
		}
		topics = append(topics, mapCategoryToTrend(category, count))
	}

	return topics, rows.Err()
}

func mapCategoryToTrend(category string, count int) TrendingTopic {
	var topic TrendingTopic
	topic.Category = category
	switch strings.ToLower(category) {
	case "gaming", "videojuegos":
		topic.Emoji = "🎮"
		topic.Title = "Gaming Week"
		topic.Subtitle = fmt.Sprintf("¡%d nuevas interacciones sobre eSports y torneos!", count)
	case "anime", "manga":
		topic.Emoji = "⛩️"
		topic.Title = "Manga Drops"
		topic.Subtitle = fmt.Sprintf("Novedades y %d comentarios sobre tus series favoritas", count)
	case "ia", "artificial intelligence", "inteligencia artificial":
		topic.Emoji = "🤖"
		topic.Title = "IA en Español"
		topic.Subtitle = fmt.Sprintf("Tendencias de prompting y %d debates sobre LLMs", count)
	case "tecnología", "tech", "programación", "programming":
		topic.Emoji = "💻"
		topic.Title = "Dev Talk"
		topic.Subtitle = fmt.Sprintf("Proyectos de código abierto y %d discusiones de arquitectura", count)
	case "debate", "opinión":
		topic.Emoji = "🌟"
		topic.Title = "Debate Semanal"
		topic.Subtitle = fmt.Sprintf("¿Cuál es el mejor enfoque? %d opiniones encontradas", count)
	default:
		title := category
		if len(title) > 0 {
			title = strings.Title(strings.ToLower(title))
		}
		topic.Emoji = "✨"
		topic.Title = title
		topic.Subtitle = fmt.Sprintf("%d conversaciones recientes en esta categoría", count)
	}
	return topic
}

// GetPersonalizedFeed retrieves nodes ranked by a personalized scoring algorithm for a given user.
func GetPersonalizedFeed(db *sql.DB, userID string, limit int) ([]Node, error) {
	query := `
		WITH user_likes_cats AS (
			SELECT n.category, COUNT(*) AS count
			FROM node_likes nl
			JOIN nodes n ON nl.node_id = n.id
			WHERE nl.user_id = $1::uuid AND n.category IS NOT NULL AND n.category <> ''
			GROUP BY n.category
		),
		user_saved_cats AS (
			SELECT n.category, COUNT(*) AS count
			FROM saved_nodes sn
			JOIN nodes n ON sn.node_id = n.id
			WHERE sn.user_id = $1::uuid AND n.category IS NOT NULL AND n.category <> ''
			GROUP BY n.category
		),
		all_user_interactions AS (
			SELECT category, count FROM user_likes_cats
			UNION ALL
			SELECT category, count FROM user_saved_cats
		),
		top_categories AS (
			SELECT category, SUM(count) AS total_interactions
			FROM all_user_interactions
			GROUP BY category
			ORDER BY total_interactions DESC
			LIMIT 3
		),
		followed_users AS (
			SELECT following_id
			FROM user_follows
			WHERE follower_id = $1::uuid AND status = 'accepted'
		),
		scored_nodes AS (
			SELECT 
				n.id, 
				COALESCE(n.slug,'') AS slug, 
				n.title, 
				COALESCE(n.description,'') AS description, 
				COALESCE(n.category,'') AS category, 
				n.image_url, 
				COALESCE(u.username, 'Desconocido') AS username, 
				n.created_at,
				(SELECT COUNT(*) FROM node_likes WHERE node_id = n.id) AS likes_count,
				EXISTS (SELECT 1 FROM node_likes WHERE node_id = n.id AND user_id = $1::uuid) AS is_liked,
				EXISTS (SELECT 1 FROM saved_nodes WHERE node_id = n.id AND user_id = $1::uuid) AS is_saved,
				CASE WHEN n.owner_id IN (SELECT following_id FROM followed_users) THEN 10 ELSE 0 END AS social_bonus,
				CASE WHEN n.category IN (SELECT category FROM top_categories) THEN 5 ELSE 0 END AS category_bonus
			FROM nodes n
			LEFT JOIN users u ON n.owner_id = u.id
		)
		SELECT id, slug, title, description, category, image_url, username, created_at, likes_count, is_liked, is_saved,
		       (1.0 + likes_count + social_bonus + category_bonus) / (1.0 + extract(epoch from (NOW() - created_at)) / 86400.0) AS final_score
		FROM scored_nodes
		ORDER BY final_score DESC
		LIMIT $2
	`

	rows, err := db.Query(query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get personalized feed: %w", err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		var finalScore float64
		err := rows.Scan(
			&n.ID, &n.Slug, &n.Title, &n.Description, &n.Category, &n.ImageURL, 
			&n.Username, &n.CreatedAt, &n.LikesCount, &n.IsLiked, &n.IsSaved, &finalScore,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan personalized node: %w", err)
		}
		if n.Username != "" {
			n.UserInitial = strings.ToUpper(string([]rune(n.Username)[0]))
		} else {
			n.UserInitial = "A"
		}
		nodes = append(nodes, n)
	}

	return nodes, rows.Err()
}



package database

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq" // PostgreSQL driver
	"log"
	"os"
)

// InitDB initializes and returns a new PostgreSQL database connection pool.
func InitDB() (*sql.DB, error) {
	// Definimos valores por defecto si las variables de entorno están vacías
	host := os.Getenv("DB_HOST")
	if host == "" { host = "localhost" }

	port := os.Getenv("DB_PORT")
	if port == "" { port = "5432" }

	user := os.Getenv("DB_USER")
	if user == "" { user = "nodal_user" }

	password := os.Getenv("DB_PASSWORD")
	if password == "" { password = "nodal_password_dev" }

	dbname := os.Getenv("DB_NAME")
	if dbname == "" { dbname = "nodal_db" }

	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Ping the database to verify the connection
	if err = db.Ping(); err != nil {
		db.Close() // Close the connection if ping fails
		return nil, fmt.Errorf("failed to connect to the database: %w", err)
	}

	log.Println("Successfully connected to the database!")

	// Run inline migrations to ensure new schema columns and tables exist
	migrations := []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS bio TEXT;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS is_private BOOLEAN DEFAULT FALSE;`,
		`ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;`,
		`CREATE TABLE IF NOT EXISTS user_follows (
			follower_id UUID REFERENCES users(id) ON DELETE CASCADE,
			following_id UUID REFERENCES users(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			PRIMARY KEY (follower_id, following_id)
		);`,
		`ALTER TABLE user_follows ADD COLUMN IF NOT EXISTS status VARCHAR(20) DEFAULT 'accepted';`,
		`CREATE TABLE IF NOT EXISTS notifications (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID REFERENCES users(id) ON DELETE CASCADE,
			actor_id UUID REFERENCES users(id) ON DELETE CASCADE,
			type VARCHAR(50) NOT NULL,
			reference_id UUID,
			is_read BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);`,
		`ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS parent_id UUID;`,
		`ALTER TABLE chat_messages DROP CONSTRAINT IF EXISTS chat_messages_parent_id_fkey;`,
		`ALTER TABLE chat_messages ADD CONSTRAINT chat_messages_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES chat_messages(id) ON DELETE CASCADE;`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			log.Printf("WARN: failed to execute schema migration query: %q, err: %v", m, err)
		}
	}

	return db, nil
}

// CloseDB closes the database connection pool.
func CloseDB(db *sql.DB) {
	if db != nil {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database connection: %v", err)
		} else {
			log.Println("Database connection closed.")
		}
	}
}

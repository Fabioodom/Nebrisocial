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

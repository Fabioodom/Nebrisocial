package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	nats "github.com/nats-io/nats.go"

	"nodal/internal/handlers"
	"nodal/internal/middleware"
	"nodal/internal/platform/database"
)

func main() {
	log.Println("Iniciando el servidor Nodal...")

	// ── Cargar variables de entorno ──────────────────────────────────────────
	if err := godotenv.Load(); err != nil {
		log.Println("WARN: No se encontró archivo .env (usando variables del sistema)")
	}

	// ── Base de datos ────────────────────────────────────────────────────────
	db, err := database.InitDB()
	if err != nil {
		log.Fatalf("CRÍTICO: No se pudo conectar a PostgreSQL: %v", err)
	}
	defer db.Close()

	// ── Conexión NATS ────────────────────────────────────────────────────────
	// NATS_URL puede dejarse vacía para entornos sin NATS (el handler lo tolera).
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL // nats://127.0.0.1:4222
	}

	var nc *nats.Conn
	nc, err = nats.Connect(
		natsURL,
		nats.Name("nodal-backend"),
		nats.MaxReconnects(-1),  // reconectar indefinidamente
		nats.ReconnectWait(2e9), // 2 s entre reintentos
	)
	if err != nil {
		// No es fatal: el handler funciona en modo degradado sin NATS.
		log.Printf("WARN: No se pudo conectar a NATS (%s): %v — el Guardián estará desactivado", natsURL, err)
		nc = nil
	} else {
		log.Printf("Conectado a NATS en %s", natsURL)
		defer nc.Drain() // flush pendientes antes de salir
	}

	// ── Router ───────────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	mux.HandleFunc("/", handlers.HomeHandler)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(); err != nil {
			http.Error(w, "Nodal Error - DB Desconectada", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Nodal OK - DB Connected"))
	})

	// ── Rutas de Autenticación ────────────────────────────────────────────────
	authHandler := handlers.NewAuthHandler(db)

	// Vistas HTML (GET) — para el navegador
	mux.HandleFunc("/login", authHandler.ShowLogin)
	mux.HandleFunc("/register", authHandler.ShowRegister)

	// API de autenticación (POST)
	mux.HandleFunc("/auth/register", authHandler.Register)
	mux.HandleFunc("/auth/login", authHandler.Login)
	mux.HandleFunc("/auth/refresh", authHandler.Refresh)
	mux.HandleFunc("/auth/logout", authHandler.Logout)

	// OAuth2 — Google
	mux.HandleFunc("/auth/google/login", authHandler.GoogleLogin)
	mux.HandleFunc("/auth/google/callback", authHandler.GoogleCallback)

	// OAuth2 — GitHub
	mux.HandleFunc("/auth/github/login", authHandler.GitHubLogin)
	mux.HandleFunc("/auth/github/callback", authHandler.GitHubCallback)

	// ── Rutas protegidas (API) ────────────────────────────────────────────────
	// /nodes protegido con RequireAuth (Bearer token en header Authorization).
	// Las peticiones HTMX desde la home ya llevan la cookie de sesión, que el
	// middleware puede leer; para clientes API se usa el header Bearer.
	mux.Handle("/nodes", middleware.RequireAuth(
		http.HandlerFunc(handlers.CreateNodeHandler(db, nc)),
	))

	// ── Arranque ─────────────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Servidor escuchando en http://localhost:%s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), mux); err != nil {
		log.Fatalf("CRÍTICO: El servidor falló: %v", err)
	}
}
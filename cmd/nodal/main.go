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
	"nodal/internal/platform/websocket"
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

	// ── WebSocket Hub ────────────────────────────────────────────────────────
	hub := websocket.NewHub()
	go hub.Run()

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

	// Archivos estáticos (CSS, JS, imágenes) — Design System Fase A
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	// Home — ahora recibe db para listar nodos
	mux.HandleFunc("/", handlers.HomeHandler(db))
	mux.HandleFunc("GET /explore", handlers.ExploreHandler(db))
	mux.HandleFunc("GET /search", handlers.SearchHandler(db))
	mux.HandleFunc("GET /components/sidebar/left", handlers.LeftSidebarHandler(db))
	mux.HandleFunc("GET /components/sidebar/right", handlers.RightSidebarHandler(db))

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
	mux.HandleFunc("/profile", handlers.ProfileHandler(db))
	mux.HandleFunc("POST /profile/edit", handlers.EditProfileHandler(db))
	mux.HandleFunc("POST /users/{id}/follow", handlers.FollowHandler(db))
	mux.HandleFunc("GET /notifications", handlers.NotificationsHandler(db))
	mux.HandleFunc("GET /notifications/all", handlers.NotificationsAllHandler(db))
	mux.HandleFunc("POST /notifications/{id}/accept", handlers.NotificationsAcceptHandler(db))
	mux.HandleFunc("POST /notifications/{id}/reject", handlers.NotificationsRejectHandler(db))
	mux.HandleFunc("GET /notifications/unread-count", handlers.NotificationsUnreadCountHandler(db))




	// API de autenticación (POST)
	mux.HandleFunc("/auth/register", authHandler.Register)
	mux.HandleFunc("/auth/login", authHandler.Login)
	mux.HandleFunc("/auth/refresh", authHandler.Refresh)
	mux.HandleFunc("/auth/logout", authHandler.Logout)

	// OAuth2 — Google
	mux.HandleFunc("GET /auth/google/login", handlers.OAuthGoogleLogin)
	mux.HandleFunc("GET /auth/google/callback", handlers.OAuthGoogleCallback)

	// OAuth2 — GitHub
	mux.HandleFunc("GET /auth/github/login", handlers.OAuthGithubLogin)
	mux.HandleFunc("GET /auth/github/callback", handlers.OAuthGithubCallback)

	// ── Rutas de Nodos ────────────────────────────────────────────────────────
	// POST /nodes — crear un nodo (protegido por RequireAuth)
	mux.Handle("POST /nodes", middleware.RequireAuth(
		http.HandlerFunc(handlers.CreateNodeHandler(db, nc)),
	))

	// GET /nodes/{id} — ver el detalle de un nodo (público)
	mux.HandleFunc("GET /nodes/{id}", handlers.NodeDetailHandler(db))

	// POST /nodes/{id}/chat — enviar un mensaje de chat (público para pruebas)
	mux.HandleFunc("POST /nodes/{id}/chat", handlers.PostChatMessageHandler(db, nc, hub))

	// GET /nodes/{id}/ws — WebSocket de tiempo real
	mux.HandleFunc("GET /nodes/{id}/ws", handlers.WebSocketHandler(hub))

	// POST /nodes/{id}/like — dar/quitar like a un nodo (protegido por RequireAuth)
	mux.Handle("POST /nodes/{id}/like", middleware.RequireAuth(
		http.HandlerFunc(handlers.NodeLikeHandler(db)),
	))

	// POST /nodes/{id}/save — guardar/quitar guardado de un nodo (protegido por RequireAuth)
	mux.Handle("POST /nodes/{id}/save", middleware.RequireAuth(
		http.HandlerFunc(handlers.ToggleSaveHandler(db)),
	))

	// ── Rutas de Administración / Auditoría ──────────────────────────────────
	mux.HandleFunc("/admin/audit", handlers.AuditHandler(db))

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
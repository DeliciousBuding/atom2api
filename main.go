package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/atom2api/atom2api/admin"
	"github.com/atom2api/atom2api/auth"
	"github.com/atom2api/atom2api/config"
	"github.com/atom2api/atom2api/database"
	"github.com/atom2api/atom2api/middleware"
	"github.com/atom2api/atom2api/proxy"
)

//go:embed frontend/dist/*
var frontendFS embed.FS

func main() {
	cfg := config.Load()

	db, err := database.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Import tokens from env var on first run
	total, _, _, _ := db.TokenCount()
	if total == 0 {
		imported, err := auth.ImportFromEnv(db)
		if err != nil {
			log.Printf("Token env import: %v", err)
		} else if imported > 0 {
			log.Printf("Imported %d token(s) from ATOMCODE_TOKENS env", imported)
		}
	}

	// Read admin secret from DB if not set via env
	if cfg.AdminSecret == "" {
		dbSecret, _ := db.GetSetting("admin_secret")
		cfg.AdminSecret = dbSecret
	}

	pool := auth.NewTokenPool(db)

	// Start token refresh loop (every 6 hours)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	auth.StartRefreshLoop(ctx, db, 6*time.Hour)

	proxyHandler := proxy.NewHandler(pool, db, cfg.UpstreamURL)
	adminHandler := admin.NewHandler(db, pool, cfg)

	mux := http.NewServeMux()

	// Public OpenAI-compatible endpoints
	publicMux := http.NewServeMux()
	publicMux.HandleFunc("POST /v1/chat/completions", proxyHandler.HandleChatCompletions)
	publicMux.HandleFunc("POST /chat/completions", proxyHandler.HandleChatCompletions)
	publicMux.HandleFunc("GET /v1/models", proxyHandler.HandleListModels)
	publicMux.HandleFunc("GET /v1/models/{id}", proxyHandler.HandleListModels)
	publicMux.HandleFunc("GET /v1/health", proxyHandler.HandleHealth)
	publicMux.HandleFunc("GET /health", proxyHandler.HandleHealth)

	rl := middleware.NewRateLimiter(cfg.RateLimitRPM)
	var publicHandler http.Handler = publicMux
	publicHandler = middleware.APIKeyAuth(db, publicHandler)
	publicHandler = rl.Middleware(publicHandler)

	// Bootstrap endpoints (no auth, always available)
	mux.HandleFunc("GET /api/admin/bootstrap-status", adminHandler.HandleBootstrapStatus)
	mux.HandleFunc("POST /api/admin/bootstrap", adminHandler.HandleBootstrap)

	// Admin endpoints — dynamic bootstrap guard
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("POST /api/admin/auth/login", adminHandler.HandleLogin)
	adminMux.HandleFunc("GET /api/admin/stats", adminHandler.HandleStats)
	adminMux.HandleFunc("GET /api/admin/tokens", adminHandler.HandleListTokens)
	adminMux.HandleFunc("POST /api/admin/tokens", adminHandler.HandleAddToken)
	adminMux.HandleFunc("POST /api/admin/tokens/import-env", adminHandler.HandleImportFromEnv)
	adminMux.HandleFunc("POST /api/admin/tokens/{id}/refresh", adminHandler.HandleRefreshToken)
	adminMux.HandleFunc("DELETE /api/admin/tokens/{id}", adminHandler.HandleDeleteToken)
	adminMux.HandleFunc("GET /api/admin/apikeys", adminHandler.HandleListAPIKeys)
	adminMux.HandleFunc("POST /api/admin/apikeys", adminHandler.HandleCreateAPIKey)
	adminMux.HandleFunc("DELETE /api/admin/apikeys/{id}", adminHandler.HandleDeleteAPIKey)
	adminMux.HandleFunc("GET /api/admin/usage", adminHandler.HandleUsage)
	adminMux.HandleFunc("GET /api/admin/settings", adminHandler.HandleGetSettings)
	adminMux.HandleFunc("PUT /api/admin/settings", adminHandler.HandleUpdateSettings)

	mux.Handle("/api/admin/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bootstrap endpoints are always open
		if r.URL.Path == "/api/admin/bootstrap-status" || r.URL.Path == "/api/admin/bootstrap" {
			return // already handled above
		}
		// Dynamic check: if admin_secret not set, return 503
		if cfg.AdminSecret == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(503)
			w.Write([]byte(`{"error":"not_bootstrapped","message":"Visit /admin/ to set up admin secret first"}`))
			return
		}
		middleware.AdminAuth(cfg.AdminSecret, adminMux).ServeHTTP(w, r)
	}))

	// Frontend
	frontendHandler := buildFrontendHandler()
	mux.Handle("/admin/", frontendHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/admin/", http.StatusFound)
			return
		}
		frontendHandler.ServeHTTP(w, r)
	})

	// Public API
	mux.Handle("/v1/", publicHandler)
	mux.Handle("/chat/", publicHandler)
	mux.Handle("/health", publicHandler)

	handler := middleware.CORS(middleware.Logger(mux))

	addr := cfg.BindAddr + ":" + cfg.Port
	srv := &http.Server{Addr: addr, Handler: handler, ReadTimeout: 30 * time.Second, WriteTimeout: 180 * time.Second}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		srv.Shutdown(shutdownCtx)
		cancel()
	}()

	printBanner(cfg, addr)

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
	log.Println("Server stopped")
}

func buildFrontendHandler() http.Handler {
	distFS, _ := fs.Sub(frontendFS, "frontend/dist")
	indexHTML, _ := fs.ReadFile(distFS, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/admin/")
		if path == "" || path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(indexHTML)
			return
		}
		f, err := distFS.Open(path)
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(indexHTML)
			return
		}
		f.Close()
		http.StripPrefix("/admin/", http.FileServer(http.FS(distFS))).ServeHTTP(w, r)
	})
}

func printBanner(cfg *config.Config, addr string) {
	secretSource := "not set"
	switch {
	case os.Getenv("ADMIN_SECRET") != "":
		secretSource = "environment"
	case cfg.AdminSecret != "":
		secretSource = "database"
	}

	bootstrapLine := ""
	if cfg.AdminSecret == "" {
		bootstrapLine = "\n║  ! First run: visit /admin/ to set admin secret    ║\n"
	}

	fmt.Printf(`
╔═══════════════════════════════════════════════════╗
║              Atom2API v1.0.0                      ║
║       OpenAI-compatible CodingPlan proxy          ║
╠═══════════════════════════════════════════════════╣
║  API:    http://%s/v1/chat/completions         ║
║  Admin:  http://%s/admin/                      ║
║  Health: http://%s/health                      ║
║  Secret: %-41s ║%s╚═══════════════════════════════════════════════════╝
`, addr, addr, addr, secretSource, bootstrapLine)
}

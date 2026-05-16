package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/atom2api/atom2api/admin"
	"github.com/atom2api/atom2api/auth"
	"github.com/atom2api/atom2api/config"
	"github.com/atom2api/atom2api/database"
	"github.com/atom2api/atom2api/middleware"
	"github.com/atom2api/atom2api/proxy"
)

func main() {
	cfg := config.Load()

	db, err := database.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	log.Println("Database opened:", cfg.DBPath)

	// Auto-import from AtomCode on first run
	total, _, _, _ := db.TokenCount()
	if total == 0 {
		imported, err := auth.ImportFromAtomCode(db, cfg.AtomCodeConfDir)
		if err != nil {
			log.Printf("Auto-import from AtomCode: %v (you can add tokens via admin UI)", err)
		} else if imported > 0 {
			log.Printf("Auto-imported %d token(s) from AtomCode", imported)
		}
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

	// Rate limiter
	rl := middleware.NewRateLimiter(cfg.RateLimitRPM)
	var publicHandler http.Handler = publicMux
	publicHandler = middleware.APIKeyAuth(db, publicHandler)
	publicHandler = rl.Middleware(publicHandler)

	// Admin endpoints
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("POST /api/admin/auth/login", adminHandler.HandleLogin)
	adminMux.HandleFunc("GET /api/admin/stats", adminHandler.HandleStats)
	adminMux.HandleFunc("GET /api/admin/tokens", adminHandler.HandleListTokens)
	adminMux.HandleFunc("POST /api/admin/tokens", adminHandler.HandleAddToken)
	adminMux.HandleFunc("POST /api/admin/tokens/import-atomcode", adminHandler.HandleImportAtomCode)
	adminMux.HandleFunc("POST /api/admin/tokens/{id}/refresh", adminHandler.HandleRefreshToken)
	adminMux.HandleFunc("DELETE /api/admin/tokens/{id}", adminHandler.HandleDeleteToken)
	adminMux.HandleFunc("GET /api/admin/apikeys", adminHandler.HandleListAPIKeys)
	adminMux.HandleFunc("POST /api/admin/apikeys", adminHandler.HandleCreateAPIKey)
	adminMux.HandleFunc("DELETE /api/admin/apikeys/{id}", adminHandler.HandleDeleteAPIKey)
	adminMux.HandleFunc("GET /api/admin/usage", adminHandler.HandleUsage)
	adminMux.HandleFunc("GET /api/admin/settings", adminHandler.HandleGetSettings)
	adminMux.HandleFunc("PUT /api/admin/settings", adminHandler.HandleUpdateSettings)

	var adminHandler2 http.Handler = adminMux
	adminHandler2 = middleware.AdminAuth(cfg.AdminSecret, adminHandler2)

	// Frontend serving
	frontendHandler := serveFrontend()

	// Root router
	mux.Handle("/api/admin/", adminHandler2)
	mux.Handle("/v1/", publicHandler)
	mux.Handle("/chat/", publicHandler)
	mux.Handle("/health", publicHandler)
	mux.Handle("/admin/", frontendHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/admin/", http.StatusFound)
			return
		}
		frontendHandler.ServeHTTP(w, r)
	})

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

	fmt.Printf(`
╔═══════════════════════════════════════════╗
║            Atom2API v1.0.0                ║
║  OpenAI-compatible proxy for AtomCode     ║
╠═══════════════════════════════════════════╣
║  API:    http://%s/v1/chat/completions ║
║  Admin:  http://%s/admin/              ║
║  Health: http://%s/health              ║
╚═══════════════════════════════════════════╝
`, addr, addr, addr)

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
	log.Println("Server stopped")
}

package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings"

	_ "github.com/lib/pq"

	"github.com/gemini-hackathon/app/internal/auth"
	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/gemini"
	"github.com/gemini-hackathon/app/internal/handlers"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := sql.Open("postgres", cfg.DBConnectionString)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	if err := storage.RunMigrations(db, "migrations"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	storageDB := storage.NewPostgresDB(db)

	fileStorage, err := storage.NewLocalFileStorage(cfg.UploadDir)
	if err != nil {
		log.Fatalf("Failed to create file storage: %v", err)
	}

	redisClient, err := storage.NewRedisClient(cfg.RedisAddr)
	if err != nil {
		log.Printf("Warning: Failed to connect to Redis: %v. OAuth state will not work.", err)
	}

	geminiClient := gemini.NewClient(cfg.GeminiAPIKey)

	tokenService := auth.NewTokenService(cfg.JWTSecret, cfg.TokenExpiryMinutes)

	googleOAuth := auth.NewGoogleOAuthService(cfg, redisClient)

	authHandlers := handlers.NewAuthHandlers(googleOAuth, tokenService, storageDB, cfg)
	userHandlers := handlers.NewUserHandlers(storageDB)
	scanHandlers := handlers.NewScanHandlers(storageDB, fileStorage, geminiClient, cfg)
	aiHandlers := handlers.NewAIHandlers(storageDB, geminiClient)
	annotationHandlers := handlers.NewAnnotationHandlers(storageDB, cfg)

	authMiddleware := middleware.NewAuthMiddleware(tokenService)

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/v1/auth/google/state", authHandlers.GoogleStateAPI)
	mux.HandleFunc("/v1/auth/google/callback", authHandlers.GoogleCallback)

	authMux := http.NewServeMux()
	authMux.HandleFunc("/v1/users/me/languages", userHandlers.GetLanguagesAPI)
	authMux.HandleFunc("/v1/users/me", userHandlers.UsersMeAPI)
	authMux.HandleFunc("/v1/scans", scanHandlers.CreateScanAPI)
	authMux.HandleFunc("/v1/scans", scanHandlers.GetScansAPI)
	authMux.HandleFunc("/v1/scans/", scanHandlers.GetScanAPI)
	authMux.HandleFunc("/v1/ai/analyze", aiHandlers.AnalyzeAPI)
	authMux.HandleFunc("/v1/annotations", annotationHandlers.CreateAnnotationAPI)
	authMux.HandleFunc("/v1/annotations", annotationHandlers.GetAnnotationsAPI)

	mux.Handle("/v1/", authMiddleware.Handle(authMux))

	reactFS := http.FileServer(http.Dir("web/dist"))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/") || strings.HasPrefix(r.URL.Path, "/healthz") {
			http.NotFound(w, r)
			return
		}

		if _, err := os.Stat("web/dist/index.html"); err == nil {
			if r.URL.Path != "/" && !strings.HasPrefix(r.URL.Path, "/v1/") && r.URL.Path != "/healthz" {
				r.URL.Path = "/"
			}
			reactFS.ServeHTTP(w, r)
		} else {
			http.Error(w, "Frontend not built. Run: cd web && bun run build", http.StatusServiceUnavailable)
		}
	})

	handler := middleware.LoggingMiddleware(mux)

	log.Printf("Server starting on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

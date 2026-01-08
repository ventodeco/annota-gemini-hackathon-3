package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

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

	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	db, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := storage.RunMigrations(db, "migrations"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	storageDB := storage.NewSQLiteDB(db)

	fileStorage, err := storage.NewLocalFileStorage(cfg.UploadDir)
	if err != nil {
		log.Fatalf("Failed to create file storage: %v", err)
	}

	geminiClient := gemini.NewClient(cfg.GeminiAPIKey)

	h, err := handlers.NewHandlers(storageDB, fileStorage, geminiClient, cfg)
	if err != nil {
		log.Fatalf("Failed to create handlers: %v", err)
	}

	sessionMiddleware := middleware.NewSessionMiddleware(storageDB, cfg.SessionCookieName, cfg.SessionSecure)

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", h.Healthz)
	mux.HandleFunc("/", h.Home)
	mux.HandleFunc("/scans", h.CreateScan)
	mux.HandleFunc("/scans/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/scans/" {
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/annotate") {
			h.Annotate(w, r)
		} else {
			h.GetScan(w, r)
		}
	})

	fs := http.FileServer(http.Dir("web/static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	handler := middleware.LoggingMiddleware(sessionMiddleware.Handle(mux))

	log.Printf("Server starting on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

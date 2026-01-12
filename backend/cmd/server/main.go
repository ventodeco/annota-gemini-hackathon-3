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

	if err := storage.RunMigrations(db, "backend/migrations"); err != nil {
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

	mux.HandleFunc("/api/scans", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			h.CreateScanAPI(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/scans/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/scans/")
		parts := strings.Split(path, "/")

		if len(parts) == 1 && parts[0] != "" {
			if r.Method == http.MethodGet {
				h.GetScanAPI(w, r)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		if len(parts) == 2 {
			switch parts[1] {
			case "annotate":
				if r.Method == http.MethodPost {
					h.AnnotateAPI(w, r)
				} else {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
				return
			case "image":
				if r.Method == http.MethodGet {
					h.GetScanImage(w, r)
				} else {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
				return
			}
		}

		http.NotFound(w, r)
	})

	reactFS := http.FileServer(http.Dir("web/dist"))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		if _, err := os.Stat("web/dist/index.html"); err == nil {
			if r.URL.Path != "/" && !strings.HasPrefix(r.URL.Path, "/api/") {
				r.URL.Path = "/"
			}
			reactFS.ServeHTTP(w, r)
		} else {
			http.Error(w, "Frontend not built. Run: cd web && bun run build", http.StatusServiceUnavailable)
		}
	})

	handler := middleware.LoggingMiddleware(sessionMiddleware.Handle(mux))

	log.Printf("Server starting on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

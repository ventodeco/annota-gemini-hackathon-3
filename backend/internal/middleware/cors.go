package middleware

import (
	"net/http"

	"github.com/gemini-hackathon/app/internal/config"
)

// CORSMiddleware adds CORS headers to allow frontend access from configured origins
type CORSMiddleware struct {
	cfg *config.Config
}

// NewCORSMiddleware creates a new CORS middleware with the given config
func NewCORSMiddleware(cfg *config.Config) *CORSMiddleware {
	return &CORSMiddleware{cfg: cfg}
}

// Handle wraps an HTTP handler with CORS functionality
func (m *CORSMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if m.cfg.IsOriginAllowed(origin) {
			if m.cfg.AllowedOrigins == "*" {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, x-token, Authorization")
			w.Header().Set("Access-Control-Max-Age", "3600")
		}

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

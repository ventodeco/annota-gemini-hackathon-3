package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gemini-hackathon/app/internal/logger"
	"github.com/gemini-hackathon/app/internal/middleware"
)

// ReadinessCheck reports whether one runtime dependency can serve requests.
type ReadinessCheck struct {
	Name  string
	Check func(context.Context) error
}

// NewReadinessHandler responds 200 only when every check succeeds within
// timeout, and 503 otherwise. Failure details are logged, never returned.
func NewReadinessHandler(timeout time.Duration, checks ...ReadinessCheck) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		for _, check := range checks {
			if err := check.Check(ctx); err != nil {
				logger.GetDefaultLogger().
					WithRequestID(middleware.GetRequestID(r.Context())).
					WithField("dependency", check.Name).
					ErrorWithErr(err, "Readiness check failed")
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("not ready"))
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	}
}

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSMiddleware_PreflightAllowedOrigin(t *testing.T) {
	tests := []struct {
		name   string
		origin string
	}{
		{
			name:   "vite localhost origin",
			origin: "http://localhost:5173",
		},
		{
			name:   "alt localhost origin",
			origin: "http://localhost:3000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
			})

			handler := CORSMiddleware(next)
			req := httptest.NewRequest(http.MethodOptions, "/v1/auth/google/state", nil)
			req.Header.Set("Origin", tt.origin)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			if nextCalled {
				t.Fatalf("next handler should not be called for OPTIONS preflight")
			}

			if recorder.Code != http.StatusNoContent {
				t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
			}

			if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != tt.origin {
				t.Fatalf("expected Access-Control-Allow-Origin %q, got %q", tt.origin, got)
			}

			if got := recorder.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PUT, PATCH, DELETE, OPTIONS" {
				t.Fatalf("unexpected Access-Control-Allow-Methods: %q", got)
			}

			if got := recorder.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, x-token, Authorization" {
				t.Fatalf("unexpected Access-Control-Allow-Headers: %q", got)
			}
		})
	}
}

func TestCORSMiddleware_NonOptionsPassesThrough(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusAccepted)
	})

	handler := CORSMiddleware(next)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/state", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if !nextCalled {
		t.Fatalf("expected next handler to be called for GET")
	}

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, recorder.Code)
	}

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("expected Access-Control-Allow-Origin to be set, got %q", got)
	}
}

func TestCORSMiddleware_DisallowedOriginDoesNotSetHeaders(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CORSMiddleware(next)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/state", nil)
	req.Header.Set("Origin", "http://example.com")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if !nextCalled {
		t.Fatalf("expected next handler to be called for disallowed origin GET")
	}

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected Access-Control-Allow-Origin to be empty, got %q", got)
	}
}

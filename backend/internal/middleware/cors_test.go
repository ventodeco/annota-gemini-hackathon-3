package middleware

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/gemini-hackathon/app/internal/config"
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

	cfg := &config.Config{AllowedOrigins: "http://localhost:5173,http://localhost:3000"}
	corsMiddleware := NewCORSMiddleware(cfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
			})

			handler := corsMiddleware.Handle(next)
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

			if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
				t.Fatalf("expected Access-Control-Allow-Credentials to be true, got %q", got)
			}

			if got := recorder.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PUT, PATCH, DELETE, OPTIONS" {
				t.Fatalf("unexpected Access-Control-Allow-Methods: %q", got)
			}

			if got := recorder.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, Authorization" {
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

	cfg := &config.Config{AllowedOrigins: "http://localhost:5173,http://localhost:3000"}
	corsMiddleware := NewCORSMiddleware(cfg)

	handler := corsMiddleware.Handle(next)
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

	cfg := &config.Config{AllowedOrigins: "http://localhost:5173,http://localhost:3000"}
	corsMiddleware := NewCORSMiddleware(cfg)

	handler := corsMiddleware.Handle(next)
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

func TestCORSMiddleware_WildcardAllowsAnyOrigin(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	cfg := &config.Config{AllowedOrigins: "*"}
	corsMiddleware := NewCORSMiddleware(cfg)

	handler := corsMiddleware.Handle(next)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/state", nil)
	req.Header.Set("Origin", "https://any-domain.com")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if !nextCalled {
		t.Fatalf("expected next handler to be called")
	}

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected Access-Control-Allow-Origin to be '*', got %q", got)
	}

	if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("expected Access-Control-Allow-Credentials to be omitted for wildcard origins, got %q", got)
	}
}

func TestCORSMiddleware_WildcardPreflightOmitsCredentials(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	cfg := &config.Config{AllowedOrigins: "*"}
	handler := NewCORSMiddleware(cfg).Handle(next)
	req := httptest.NewRequest(http.MethodOptions, "/v1/auth/google/state", nil)
	req.Header.Set("Origin", "https://any-domain.com")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if nextCalled {
		t.Fatalf("next handler should not be called for OPTIONS preflight")
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected Access-Control-Allow-Origin to be '*', got %q", got)
	}
	assertHeaderAbsent(t, recorder.Header(), "Access-Control-Allow-Credentials")
	if varyHasOrigin(recorder.Header()) {
		t.Fatalf("expected no Vary: Origin in wildcard mode, got %q", recorder.Header().Values("Vary"))
	}
}

func TestCORSMiddleware_CredentialsRequireConcreteOrigin(t *testing.T) {
	tests := []struct {
		name           string
		allowedOrigins string
		method         string
		origin         string
		wantOrigin     string
	}{
		{
			name:           "literal wildcard entry in list denies GET",
			allowedOrigins: "https://app.example,*",
			method:         http.MethodGet,
			origin:         "*",
		},
		{
			name:           "literal wildcard entry in list denies preflight",
			allowedOrigins: "https://app.example,*",
			method:         http.MethodOptions,
			origin:         "*",
		},
		{
			name:           "concrete origin in list with wildcard entry is reflected",
			allowedOrigins: "https://app.example,*",
			method:         http.MethodGet,
			origin:         "https://app.example",
			wantOrigin:     "https://app.example",
		},
		{
			name:           "empty entry does not match missing origin",
			allowedOrigins: "https://app.example,",
			method:         http.MethodGet,
		},
		{
			name:           "empty entry does not match missing origin on preflight",
			allowedOrigins: "https://app.example,",
			method:         http.MethodOptions,
		},
		{
			name:           "unlisted origin is denied",
			allowedOrigins: "https://app.example",
			method:         http.MethodGet,
			origin:         "https://evil.example",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
			handler := NewCORSMiddleware(&config.Config{AllowedOrigins: tt.allowedOrigins}).Handle(next)
			req := httptest.NewRequest(tt.method, "/v1/scans", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			header := recorder.Header()
			if !varyHasOrigin(header) {
				t.Errorf("expected Vary to include Origin, got %q", header.Values("Vary"))
			}

			if tt.wantOrigin == "" {
				for _, name := range []string{
					"Access-Control-Allow-Origin",
					"Access-Control-Allow-Credentials",
					"Access-Control-Allow-Methods",
					"Access-Control-Allow-Headers",
					"Access-Control-Max-Age",
				} {
					assertHeaderAbsent(t, header, name)
				}
				return
			}

			if got := header.Values("Access-Control-Allow-Origin"); !slices.Equal(got, []string{tt.wantOrigin}) {
				t.Errorf("expected Access-Control-Allow-Origin %q, got %q", tt.wantOrigin, got)
			}
			if got := header.Values("Access-Control-Allow-Credentials"); !slices.Equal(got, []string{"true"}) {
				t.Errorf("expected Access-Control-Allow-Credentials true, got %q", got)
			}
		})
	}
}

func TestCORSMiddleware_PreservesExistingVary(t *testing.T) {
	setVary := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Vary", "Accept-Encoding")
			next.ServeHTTP(w, r)
		})
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	handler := setVary(NewCORSMiddleware(&config.Config{AllowedOrigins: "https://app.example"}).Handle(next))
	req := httptest.NewRequest(http.MethodGet, "/v1/scans", nil)
	req.Header.Set("Origin", "https://app.example")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if !varyHasToken(recorder.Header(), "Accept-Encoding") {
		t.Fatalf("expected existing Vary value to be preserved, got %q", recorder.Header().Values("Vary"))
	}
	if !varyHasOrigin(recorder.Header()) {
		t.Fatalf("expected Vary to include Origin, got %q", recorder.Header().Values("Vary"))
	}
}

func assertHeaderAbsent(t *testing.T, header http.Header, name string) {
	t.Helper()
	if values := header.Values(name); len(values) != 0 {
		t.Errorf("expected %s to be absent, got %q", name, values)
	}
}

func varyHasOrigin(header http.Header) bool {
	return varyHasToken(header, "Origin")
}

func varyHasToken(header http.Header, token string) bool {
	for _, value := range header.Values("Vary") {
		for _, part := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return true
			}
		}
	}
	return false
}

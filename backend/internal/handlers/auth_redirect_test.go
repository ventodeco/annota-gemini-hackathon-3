package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/handlers"
)

func TestGoogleCallbackRedirect_UsesFrontendBaseURL(t *testing.T) {
	cfg := &config.Config{
		AppBaseURL:      "http://localhost:8080",
		FrontendBaseURL: "http://localhost:5173",
	}
	authHandlers := handlers.NewAuthHandlers(nil, nil, nil, cfg)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?state=s&code=c", nil)
	rec := httptest.NewRecorder()

	authHandlers.GoogleCallbackRedirect(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}

	expectedLocation := "http://localhost:5173/auth/callback?state=s&code=c"
	if got := rec.Header().Get("Location"); got != expectedLocation {
		t.Fatalf("expected redirect location %q, got %q", expectedLocation, got)
	}
}

func TestGoogleCallbackRedirect_FallsBackToAppBaseURL(t *testing.T) {
	cfg := &config.Config{
		AppBaseURL:      "http://localhost:8080",
		FrontendBaseURL: "",
	}
	authHandlers := handlers.NewAuthHandlers(nil, nil, nil, cfg)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?state=s&code=c", nil)
	rec := httptest.NewRecorder()

	authHandlers.GoogleCallbackRedirect(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}

	expectedLocation := "http://localhost:8080/auth/callback?state=s&code=c"
	if got := rec.Header().Get("Location"); got != expectedLocation {
		t.Fatalf("expected redirect location %q, got %q", expectedLocation, got)
	}
}

func TestGoogleCallbackRedirect_MissingStateOrCode(t *testing.T) {
	cfg := &config.Config{
		AppBaseURL:      "http://localhost:8080",
		FrontendBaseURL: "http://localhost:5173",
	}
	authHandlers := handlers.NewAuthHandlers(nil, nil, nil, cfg)

	tests := []struct {
		name string
		url  string
	}{
		{
			name: "missing state",
			url:  "/v1/auth/google/callback?code=c",
		},
		{
			name: "missing code",
			url:  "/v1/auth/google/callback?state=s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rec := httptest.NewRecorder()

			authHandlers.GoogleCallbackRedirect(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}
		})
	}
}

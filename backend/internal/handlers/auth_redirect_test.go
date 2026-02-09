package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
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

	expectedLocation := "http://localhost:5173/auth/callback?code=c&state=s"
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

	expectedLocation := "http://localhost:8080/auth/callback?code=c&state=s"
	if got := rec.Header().Get("Location"); got != expectedLocation {
		t.Fatalf("expected redirect location %q, got %q", expectedLocation, got)
	}
}

func TestGoogleCallbackRedirect_EncodesCodeQueryParam(t *testing.T) {
	cfg := &config.Config{
		AppBaseURL:      "http://localhost:8080",
		FrontendBaseURL: "http://localhost:5173",
	}
	authHandlers := handlers.NewAuthHandlers(nil, nil, nil, cfg)

	encodedCode := "4%2F0ASc3gC%2Babc%3D%3D"
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?state=s&code="+encodedCode, nil)
	rec := httptest.NewRecorder()

	authHandlers.GoogleCallbackRedirect(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}

	location := rec.Header().Get("Location")
	if location != "http://localhost:5173/auth/callback?code=4%2F0ASc3gC%2Babc%3D%3D&state=s" {
		t.Fatalf("expected encoded redirect location, got %q", location)
	}

	parsedLocation, err := url.Parse(location)
	if err != nil {
		t.Fatalf("failed to parse redirect location: %v", err)
	}

	if got := parsedLocation.Query().Get("code"); got != "4/0ASc3gC+abc==" {
		t.Fatalf("expected decoded code %q, got %q", "4/0ASc3gC+abc==", got)
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

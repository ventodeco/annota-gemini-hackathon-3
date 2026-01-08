package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/testutil"
)

func TestHealthz(t *testing.T) {
	mockDB := &testutil.MockDB{}
	mockFileStorage := &testutil.MockFileStorage{}
	mockGeminiClient := &testutil.MockGeminiClient{}
	cfg := &config.Config{
		MaxUploadSize: 10 * 1024 * 1024,
	}

	h, err := NewHandlers(mockDB, mockFileStorage, mockGeminiClient, cfg)
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()

	h.Healthz(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	if rr.Body.String() != "ok" {
		t.Errorf("Expected body 'ok', got %s", rr.Body.String())
	}
}

func TestHome(t *testing.T) {
	mockDB := &testutil.MockDB{}
	mockFileStorage := &testutil.MockFileStorage{}
	mockGeminiClient := &testutil.MockGeminiClient{}
	cfg := &config.Config{
		MaxUploadSize: 10 * 1024 * 1024,
	}

	h, err := NewHandlers(mockDB, mockFileStorage, mockGeminiClient, cfg)
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	h.Home(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !bytes.Contains([]byte(body), []byte("htmx")) && !bytes.Contains([]byte(body), []byte("tailwind")) {
		t.Log("Warning: Template may not include HTMX or Tailwind markers")
	}
}

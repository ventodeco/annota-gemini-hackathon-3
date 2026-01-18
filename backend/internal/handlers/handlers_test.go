package handlers

import (
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

	h := NewHandlers(mockDB, mockFileStorage, mockGeminiClient, cfg)

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

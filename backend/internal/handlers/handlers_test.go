package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/models"
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

func TestGetScan_HTMXReturnsFragment(t *testing.T) {
	scanID := "scan123"
	mockDB := &testutil.MockDB{
		GetScanWithOCRFunc: func(ctx context.Context, id string) (*models.Scan, *models.OCRResult, error) {
			if id != scanID {
				t.Fatalf("expected scanID %q, got %q", scanID, id)
			}
			return &models.Scan{
				ID:        scanID,
				SessionID: "session1",
				Source:    "upload",
				Status:    "uploaded",
				CreatedAt: time.Now(),
			}, nil, nil
		},
	}
	mockFileStorage := &testutil.MockFileStorage{}
	mockGeminiClient := &testutil.MockGeminiClient{}
	cfg := &config.Config{
		MaxUploadSize: 10 * 1024 * 1024,
	}

	h, err := NewHandlers(mockDB, mockFileStorage, mockGeminiClient, cfg)
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	req := httptest.NewRequest("GET", "/scans/"+scanID, nil)
	req.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()

	h.GetScan(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if strings.Contains(body, "<!DOCTYPE html>") {
		t.Fatalf("Expected fragment HTML, got full page HTML")
	}
	if !strings.Contains(body, "id=\"ocr-status-container\"") {
		t.Fatalf("Expected ocr status container fragment, got: %s", body)
	}
}

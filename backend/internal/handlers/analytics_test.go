package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gemini-hackathon/app/internal/handlers"
	"github.com/gemini-hackathon/app/internal/middleware"
)

func TestAnalyticsEventAPIRequiresAuthAndAcceptsEvent(t *testing.T) {
	handler := handlers.NewAnalyticsHandlers()

	req := httptest.NewRequest(http.MethodPost, "/v1/events", strings.NewReader(`{"name":"first_annotation","properties":{"source":"pdf"}}`))
	req = req.WithContext(middleware.WithUserID(req.Context(), 7))
	rec := httptest.NewRecorder()

	handler.EventsAPI(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAnalyticsEventAPIRejectsMissingName(t *testing.T) {
	handler := handlers.NewAnalyticsHandlers()

	req := httptest.NewRequest(http.MethodPost, "/v1/events", strings.NewReader(`{"properties":{"source":"pdf"}}`))
	req = req.WithContext(middleware.WithUserID(req.Context(), 7))
	rec := httptest.NewRecorder()

	handler.EventsAPI(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAnalyticsEventAPIRejectsUnknownEvent(t *testing.T) {
	handler := handlers.NewAnalyticsHandlers()

	req := httptest.NewRequest(http.MethodPost, "/v1/events", strings.NewReader(`{"name":"ocr_text","properties":{"source":"pdf"}}`))
	req = req.WithContext(middleware.WithUserID(req.Context(), 7))
	rec := httptest.NewRecorder()

	handler.EventsAPI(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAnalyticsEventAPIRejectsUnknownProperties(t *testing.T) {
	handler := handlers.NewAnalyticsHandlers()

	req := httptest.NewRequest(http.MethodPost, "/v1/events", strings.NewReader(`{"name":"reader_activation","properties":{"selectedText":"秘密"}}`))
	req = req.WithContext(middleware.WithUserID(req.Context(), 7))
	rec := httptest.NewRecorder()

	handler.EventsAPI(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAnalyticsEventAPIRejectsOversizedPayload(t *testing.T) {
	handler := handlers.NewAnalyticsHandlers()
	body := `{"name":"reader_activation","properties":{"source":"` + strings.Repeat("x", 4096) + `"}}`

	req := httptest.NewRequest(http.MethodPost, "/v1/events", strings.NewReader(body))
	req = req.WithContext(middleware.WithUserID(req.Context(), 7))
	rec := httptest.NewRecorder()

	handler.EventsAPI(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

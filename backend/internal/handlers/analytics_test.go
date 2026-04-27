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

package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/handlers"
	"github.com/gemini-hackathon/app/internal/middleware"
)

func TestEntitlementsAPIReturnsCurrentPlanAndLimits(t *testing.T) {
	handler := handlers.NewEntitlementHandlers(&config.Config{
		AIRateLimit:              60,
		AIRateLimitWindowSeconds: 3600,
		MaxUploadSize:            10485760,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/entitlements/me", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), 3))
	rec := httptest.NewRecorder()

	handler.MeAPI(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var body handlers.EntitlementResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Plan != "free" {
		t.Fatalf("expected free plan, got %q", body.Plan)
	}
	if body.Limits.AIRequestsPerWindow != 60 {
		t.Fatalf("expected AI limit 60, got %d", body.Limits.AIRequestsPerWindow)
	}
}

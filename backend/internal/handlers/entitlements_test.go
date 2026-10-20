package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/handlers"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/usage"
)

func TestEntitlementsAPIReturnsCurrentPlanAndLimits(t *testing.T) {
	handler := handlers.NewEntitlementHandlers(&config.Config{
		AIRateLimit:              60,
		AIRateLimitWindowSeconds: 3600,
		MaxUploadSize:            10485760,
	}, newTestMeter(t))

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
	if body.Usage.Period != "daily" || body.Usage.Timezone != "UTC" || len(body.Usage.Features) != 3 {
		t.Fatalf("unexpected usage block: %+v", body.Usage)
	}
	if _, err := time.Parse(time.RFC3339, body.Usage.ResetAt); err != nil {
		t.Fatalf("resetAt %q is not RFC3339: %v", body.Usage.ResetAt, err)
	}
}

func TestEntitlementsAPIReportsRemainingAllowance(t *testing.T) {
	meter, _ := newMeterWithStore(usage.Limits{
		usage.FeatureOCR:         {User: 20, Global: 1},
		usage.FeatureExplanation: {User: 5, Global: 50},
		usage.FeatureSpeech:      {User: 3000, Global: 75000, PerRequest: 500},
	})
	if _, _, err := meter.Reserve(context.Background(), 3, usage.FeatureOCR, 1); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	handler := handlers.NewEntitlementHandlers(&config.Config{}, meter)
	req := httptest.NewRequest(http.MethodGet, "/v1/entitlements/me", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), 3))
	rec := httptest.NewRecorder()
	handler.MeAPI(rec, req)

	var body handlers.EntitlementResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ocr, speech := body.Usage.Features[0], body.Usage.Features[2]
	if ocr.Feature != "ocr" || ocr.Unit != "page" || ocr.Limit != 20 || ocr.Used != 1 || ocr.Remaining != 19 || ocr.GlobalAvailable {
		t.Fatalf("ocr usage = %+v", ocr)
	}
	if speech.MaxPerRequest == nil || *speech.MaxPerRequest != 500 || !speech.GlobalAvailable {
		t.Fatalf("speech usage = %+v", speech)
	}
}

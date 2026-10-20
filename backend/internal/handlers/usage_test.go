package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gemini-hackathon/app/internal/ai"
	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/handlers"
	"github.com/gemini-hackathon/app/internal/knowledge"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/storage"
	"github.com/gemini-hackathon/app/internal/testutil"
	"github.com/gemini-hackathon/app/internal/usage"
)

// newTestMeter gives handler tests a real meter with generous limits, so a
// nil meter never stands in for "unmetered".
func newTestMeter(t *testing.T) *usage.Meter {
	t.Helper()
	meter, _ := newMeterWithStore(usage.Limits{
		usage.FeatureOCR:         {User: 100, Global: 1000},
		usage.FeatureExplanation: {User: 100, Global: 1000},
		usage.FeatureSpeech:      {User: 10000, Global: 100000, PerRequest: 5000},
	})
	return meter
}

func newMeterWithStore(limits usage.Limits) (*usage.Meter, *testutil.MemoryUsageStore) {
	store := testutil.NewMemoryUsageStore()
	return usage.NewMeter(store, limits), store
}

// countingAIClient records provider calls so tests can prove rejected
// admissions never reach the provider.
type countingAIClient struct {
	mockSpeechAIClient
	speechCalls atomic.Int32
	annCalls    atomic.Int32
	ocrErr      error
}

func (c *countingAIClient) SynthesizeSpeech(ctx context.Context, text, contextText string) (*ai.SpeechResponse, error) {
	c.speechCalls.Add(1)
	return c.mockSpeechAIClient.SynthesizeSpeech(ctx, text, contextText)
}

func (c *countingAIClient) AnnotateWithKnowledge(ctx context.Context, ocrText, selectedText string, entries []knowledge.Entry) (*ai.AnnotationResponse, error) {
	c.annCalls.Add(1)
	return c.mockSpeechAIClient.AnnotateWithKnowledge(ctx, ocrText, selectedText, entries)
}

func (c *countingAIClient) OCR(ctx context.Context, imageData []byte, mimeType string) (*ai.OCRResponse, error) {
	if c.ocrErr != nil {
		return nil, c.ocrErr
	}
	return &ai.OCRResponse{RawText: "OCR text", Language: "JP"}, nil
}

func postJSON(userID int64, path, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	return req.WithContext(middleware.WithUserID(req.Context(), userID))
}

func decodeUsageError(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v (%s)", err, rec.Body.String())
	}
	return body
}

func TestSpeakAPIEnforcesSpeechAllowance(t *testing.T) {
	meter, store := newMeterWithStore(usage.Limits{usage.FeatureSpeech: {User: 10, Global: 100, PerRequest: 8}})
	client := &countingAIClient{mockSpeechAIClient: mockSpeechAIClient{resp: &ai.SpeechResponse{Audio: []byte("a"), MIMEType: "audio/mpeg"}}}
	h := handlers.NewAIHandlers(testutil.NewMockDB(), client, knowledge.NewEmptyService(), meter)

	rec := httptest.NewRecorder()
	h.SpeakAPI(rec, postJSON(1, "/v1/ai/speech", `{"highlightedText":"テストテスト"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("first speech = %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.SpeakAPI(rec, postJSON(1, "/v1/ai/speech", `{"highlightedText":"もう一回です"}`))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over-quota speech = %d %s", rec.Code, rec.Body.String())
	}
	body := decodeUsageError(t, rec)
	if body["code"] != "usage_quota_exceeded" || body["feature"] != "speech" || body["remaining"] != float64(4) || body["limit"] != float64(10) {
		t.Fatalf("quota body = %v", body)
	}
	if _, err := time.Parse(time.RFC3339, body["resetAt"].(string)); err != nil || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("missing reset details: %v retry-after=%q", body["resetAt"], rec.Header().Get("Retry-After"))
	}

	rec = httptest.NewRecorder()
	h.SpeakAPI(rec, postJSON(1, "/v1/ai/speech", `{"highlightedText":"あいうえおかきくけ"}`))
	if rec.Code != http.StatusRequestEntityTooLarge || decodeUsageError(t, rec)["code"] != "usage_request_too_large" {
		t.Fatalf("oversized speech = %d %s", rec.Code, rec.Body.String())
	}

	if calls := client.speechCalls.Load(); calls != 1 {
		t.Fatalf("provider speech calls = %d, want 1", calls)
	}
	if got := store.Reservations(); len(got) != 1 || got[0].Units != 6 || got[0].Status != storage.UsageStatusSettled {
		t.Fatalf("reservations = %+v", got)
	}
}

func TestSpeakAPIRefundsFailedProviderCalls(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantCode   int
		wantStatus string
		wantBody   string
	}{
		{"provider limit is a clean rejection", &ai.ProviderError{Kind: ai.ErrorKindQuota, StatusCode: 429}, http.StatusTooManyRequests, storage.UsageStatusReleased, "ai_provider_limited"},
		{"timeout may still be billed", context.DeadlineExceeded, http.StatusInternalServerError, storage.UsageStatusFailed, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meter, store := newMeterWithStore(usage.Limits{usage.FeatureSpeech: {User: 10, Global: 100}})
			h := handlers.NewAIHandlers(testutil.NewMockDB(), &mockSpeechAIClient{err: tt.err}, knowledge.NewEmptyService(), meter)
			rec := httptest.NewRecorder()
			h.SpeakAPI(rec, postJSON(1, "/v1/ai/speech", `{"highlightedText":"テスト"}`))
			if rec.Code != tt.wantCode {
				t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
			}
			if tt.wantBody != "" && decodeUsageError(t, rec)["code"] != tt.wantBody {
				t.Fatalf("body = %s", rec.Body.String())
			}
			if got := store.Reservations()[0].Status; got != tt.wantStatus {
				t.Fatalf("reservation status = %s, want %s", got, tt.wantStatus)
			}
			snapshot, _ := meter.Snapshot(context.Background(), 1)
			if snapshot.Features[2].Used != 0 {
				t.Fatalf("user was charged for a failed call: %+v", snapshot.Features[2])
			}
		})
	}
}

func TestAnalyzeAPIGlobalBudgetHidesTotals(t *testing.T) {
	db := testutil.NewMockDB()
	for _, email := range []string{"a@example.com", "b@example.com"} {
		if err := db.CreateUser(context.Background(), &models.User{Email: email, Provider: "clerk", ProviderID: email}); err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
	}
	meter, _ := newMeterWithStore(usage.Limits{usage.FeatureExplanation: {User: 5, Global: 1}})
	client := &countingAIClient{mockSpeechAIClient: mockSpeechAIClient{annotation: &ai.AnnotationResponse{Meaning: "m"}}}
	h := handlers.NewAIHandlers(db, client, knowledge.NewEmptyService(), meter)

	rec := httptest.NewRecorder()
	h.AnalyzeAPI(rec, postJSON(1, "/v1/ai/analyze", `{"textToAnalyze":"本"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("first analyze = %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h.AnalyzeAPI(rec, postJSON(2, "/v1/ai/analyze", `{"textToAnalyze":"本"}`))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("global-exhausted analyze = %d %s", rec.Code, rec.Body.String())
	}
	body := decodeUsageError(t, rec)
	if body["code"] != "usage_budget_exhausted" || body["feature"] != "explanation" {
		t.Fatalf("budget body = %v", body)
	}
	for _, key := range []string{"limit", "used", "remaining"} {
		if _, ok := body[key]; ok {
			t.Fatalf("global budget error exposes %q: %v", key, body)
		}
	}
	if calls := client.annCalls.Load(); calls != 1 {
		t.Fatalf("provider annotate calls = %d, want 1", calls)
	}
}

func TestAnalyzeAPIDoesNotChargeRejectedOwnership(t *testing.T) {
	db := testutil.NewMockDB()
	for _, email := range []string{"owner@example.com", "other@example.com"} {
		if err := db.CreateUser(context.Background(), &models.User{Email: email, Provider: "clerk", ProviderID: email}); err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
	}
	scanID, err := db.CreateScan(context.Background(), &models.Scan{UserID: 1})
	if err != nil {
		t.Fatalf("CreateScan: %v", err)
	}
	meter, store := newMeterWithStore(usage.Limits{usage.FeatureExplanation: {User: 5, Global: 5}})
	h := handlers.NewAIHandlers(db, &mockSpeechAIClient{}, knowledge.NewEmptyService(), meter)

	rec := httptest.NewRecorder()
	h.AnalyzeAPI(rec, postJSON(2, "/v1/ai/analyze", `{"textToAnalyze":"本","scanId":`+jsonInt(scanID)+`}`))
	if rec.Code != http.StatusForbidden || len(store.Reservations()) != 0 {
		t.Fatalf("forbidden analyze = %d, reservations=%d", rec.Code, len(store.Reservations()))
	}
}

func jsonInt(v int64) string {
	data, _ := json.Marshal(v)
	return string(data)
}

type failingSaveStorage struct{ mockFileStorage }

func (*failingSaveStorage) SaveImage(string, []byte, string) (string, *string, error) {
	return "", nil, errors.New("disk full")
}

func TestCreateScanAPIAdmissionAndSettlement(t *testing.T) {
	cfg := &config.Config{MaxUploadSize: 10 * 1024 * 1024}
	upload := func(h *handlers.ScanHandlers) *httptest.ResponseRecorder {
		req := buildUploadRequest(t, "/v1/scans")
		req = req.WithContext(middleware.WithUserID(req.Context(), 1))
		rec := httptest.NewRecorder()
		h.CreateScanAPI(rec, req)
		return rec
	}

	t.Run("save failure releases the page", func(t *testing.T) {
		meter, store := newMeterWithStore(usage.Limits{usage.FeatureOCR: {User: 1, Global: 10}})
		h := handlers.NewScanHandlers(testutil.NewMockDB(), &failingSaveStorage{}, &countingAIClient{}, cfg, meter)
		if rec := upload(h); rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rec.Code)
		}
		if got := store.Reservations()[0].Status; got != storage.UsageStatusReleased {
			t.Fatalf("status = %s, want released", got)
		}
	})

	t.Run("quota is checked before the scan row exists", func(t *testing.T) {
		db := testutil.NewMockDB()
		meter, _ := newMeterWithStore(usage.Limits{usage.FeatureOCR: {User: 0, Global: 10}})
		h := handlers.NewScanHandlers(db, &mockFileStorage{}, &countingAIClient{}, cfg, meter)
		rec := upload(h)
		if rec.Code != http.StatusTooManyRequests || decodeUsageError(t, rec)["code"] != "usage_quota_exceeded" {
			t.Fatalf("zero-allowance upload = %d %s", rec.Code, rec.Body.String())
		}
		if scans, _ := db.GetScansByUserID(context.Background(), 1, 1, 10); len(scans) != 0 {
			t.Fatalf("rejected upload created %d scans", len(scans))
		}
	})

	for _, tc := range []struct {
		name   string
		ocrErr error
		want   string
	}{
		{"successful OCR keeps the charge", nil, storage.UsageStatusSettled},
		{"upstream OCR failure refunds the user", &ai.ProviderError{Kind: ai.ErrorKindUpstream, StatusCode: 502}, storage.UsageStatusFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			meter, store := newMeterWithStore(usage.Limits{usage.FeatureOCR: {User: 5, Global: 10}})
			h := handlers.NewScanHandlers(testutil.NewMockDB(), &mockFileStorage{}, &countingAIClient{ocrErr: tc.ocrErr}, cfg, meter)
			if rec := upload(h); rec.Code != http.StatusCreated {
				t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
			}
			deadline := time.Now().Add(2 * time.Second)
			for store.Reservations()[0].Status == storage.UsageStatusReserved && time.Now().Before(deadline) {
				time.Sleep(5 * time.Millisecond)
			}
			if got := store.Reservations()[0].Status; got != tc.want {
				t.Fatalf("status = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestUsageStoreUnavailableFailsClosed(t *testing.T) {
	meter, store := newMeterWithStore(usage.Limits{usage.FeatureSpeech: {User: 10, Global: 10}})
	store.Err = errors.New("connection refused")
	client := &countingAIClient{}
	h := handlers.NewAIHandlers(testutil.NewMockDB(), client, knowledge.NewEmptyService(), meter)

	rec := httptest.NewRecorder()
	h.SpeakAPI(rec, postJSON(1, "/v1/ai/speech", `{"highlightedText":"テスト"}`))
	if rec.Code != http.StatusServiceUnavailable || decodeUsageError(t, rec)["code"] != "usage_unavailable" {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	if client.speechCalls.Load() != 0 {
		t.Fatal("provider was called without an admitted reservation")
	}

	entitlements := handlers.NewEntitlementHandlers(&config.Config{}, meter)
	req := httptest.NewRequest(http.MethodGet, "/v1/entitlements/me", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), 1))
	rec = httptest.NewRecorder()
	entitlements.MeAPI(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("entitlements status = %d", rec.Code)
	}
}

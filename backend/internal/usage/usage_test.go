package usage

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/gemini-hackathon/app/internal/ai"
	"github.com/gemini-hackathon/app/internal/storage"
	"github.com/gemini-hackathon/app/internal/testutil"
)

var testNow = time.Date(2026, 10, 20, 23, 30, 0, 0, time.UTC)

func newTestMeter(limits Limits) (*Meter, *testutil.MemoryUsageStore, *time.Time) {
	store := testutil.NewMemoryUsageStore()
	now := testNow
	return NewMeterWithClock(store, limits, func() time.Time { return now }), store, &now
}

func int64p(v int64) *int64 { return &v }

func TestReserveRejectsWithQuotaAndTooLargeErrors(t *testing.T) {
	meter, _, _ := newTestMeter(Limits{
		FeatureOCR:         {User: 0, Global: 10},
		FeatureExplanation: {User: 2, Global: 1},
		FeatureSpeech:      {User: 100, Global: 1000},
	})
	ctx := context.Background()

	_, _, err := meter.Reserve(ctx, 1, FeatureOCR, 1)
	var quotaErr *QuotaError
	if !errors.As(err, &quotaErr) || quotaErr.Global || quotaErr.Limit != 0 || quotaErr.Remaining() != 0 {
		t.Fatalf("zero allowance error = %#v", err)
	}
	if want := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC); !quotaErr.ResetAt.Equal(want) {
		t.Fatalf("ResetAt = %v, want %v", quotaErr.ResetAt, want)
	}

	if _, _, err := meter.Reserve(ctx, 1, FeatureExplanation, 1); err != nil {
		t.Fatalf("first explanation: %v", err)
	}
	_, _, err = meter.Reserve(ctx, 2, FeatureExplanation, 1)
	if !errors.As(err, &quotaErr) || !quotaErr.Global || quotaErr.Used != 0 {
		t.Fatalf("global exhaustion error = %#v (global totals must not leak)", err)
	}

	_, _, err = meter.Reserve(ctx, 1, FeatureSpeech, 101)
	var tooLarge *TooLargeError
	if !errors.As(err, &tooLarge) || tooLarge.Max != 100 {
		t.Fatalf("oversized speech error = %#v", err)
	}
}

func TestReserveTooLargeNeverRevealsGlobalBudget(t *testing.T) {
	meter, _, _ := newTestMeter(Limits{FeatureSpeech: {User: 3000, Global: 200, PerRequest: 500}})
	_, _, err := meter.Reserve(context.Background(), 1, FeatureSpeech, 201)
	var quotaErr *QuotaError
	if !errors.As(err, &quotaErr) || !quotaErr.Global {
		t.Fatalf("request above the global budget = %#v, want global budget error", err)
	}
	_, _, err = meter.Reserve(context.Background(), 1, FeatureSpeech, 501)
	var tooLarge *TooLargeError
	if !errors.As(err, &tooLarge) || tooLarge.Max != 500 {
		t.Fatalf("request above the per-request cap = %#v, want max 500", err)
	}
}

func TestFinishClassifiesFromStructuredAttempts(t *testing.T) {
	upstream := &ai.ProviderError{Kind: ai.ErrorKindUpstream}
	rateLimited := &ai.ProviderError{Kind: ai.ErrorKindRateLimit}
	tests := []struct {
		name       string
		calls      []ai.CallMetadata
		err        error
		wantStatus string
	}{
		{"success", []ai.CallMetadata{{}}, nil, storage.UsageStatusSettled},
		{"all attempts cleanly rejected", []ai.CallMetadata{{ErrorKind: ai.ErrorKindRateLimit}, {ErrorKind: ai.ErrorKindQuota}}, rateLimited, storage.UsageStatusReleased},
		{"earlier upstream attempt may be billed", []ai.CallMetadata{{ErrorKind: ai.ErrorKindUpstream}, {ErrorKind: ai.ErrorKindRateLimit}}, rateLimited, storage.UsageStatusFailed},
		{"response arrived but result unusable", []ai.CallMetadata{{}}, errors.New("save failed"), storage.UsageStatusFailed},
		{"legacy client retries may hide a billed attempt", nil, rateLimited, storage.UsageStatusFailed},
		{"legacy client configuration error before dispatch", nil, &ai.ProviderError{Kind: ai.ErrorKindConfig}, storage.UsageStatusReleased},
		{"legacy client configuration error after a response", nil, &ai.ProviderError{Kind: ai.ErrorKindConfig, StatusCode: 400}, storage.UsageStatusFailed},
		{"legacy client upstream failure", nil, upstream, storage.UsageStatusFailed},
		{"deadline exceeded", nil, context.DeadlineExceeded, storage.UsageStatusFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meter, store, _ := newTestMeter(Limits{FeatureExplanation: {User: 5, Global: 5}})
			ctx, res, err := meter.Reserve(context.Background(), 1, FeatureExplanation, 1)
			if err != nil {
				t.Fatalf("reserve: %v", err)
			}
			for _, call := range tt.calls {
				RecordCall(ctx, call)
			}
			if got := Calls(ctx); len(got) != len(tt.calls) {
				t.Fatalf("Calls = %d attempts, want %d", len(got), len(tt.calls))
			}
			meter.Finish(ctx, res, tt.err)
			if got := store.Reservations()[0].Status; got != tt.wantStatus {
				t.Fatalf("status = %s, want %s", got, tt.wantStatus)
			}
		})
	}
}

func TestFinishSettlesOnceAfterCancelledContext(t *testing.T) {
	meter, store, _ := newTestMeter(Limits{FeatureSpeech: {User: 10, Global: 10}})
	reqCtx, cancel := context.WithCancel(context.Background())
	ctx, res, err := meter.Reserve(reqCtx, 1, FeatureSpeech, 4)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	cancel()
	meter.Finish(ctx, res, &ai.ProviderError{Kind: ai.ErrorKindConfig})
	meter.Finish(ctx, res, nil)
	meter.Release(ctx, res)

	reservations := store.Reservations()
	if reservations[0].Status != storage.UsageStatusReleased {
		t.Fatalf("status = %s, want released despite cancelled request", reservations[0].Status)
	}
	snapshot, err := meter.Snapshot(context.Background(), 1)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if speech := snapshot.Features[2]; speech.Used != 0 || speech.Remaining != 10 {
		t.Fatalf("speech usage after single refund = %+v", speech)
	}
}

func TestSummarizeCallsReportsOnlyProviderCost(t *testing.T) {
	costA, costB := json.Number("0.0010"), json.Number("0.00025")
	reported := summarizeCalls([]ai.CallMetadata{
		{Gateway: "openrouter", Region: "global", RequestedModel: "req", InputTokens: int64p(10), Cost: &costA, Currency: "USD", ErrorKind: ai.ErrorKindRateLimit},
		{Gateway: "openrouter", Region: "global", RequestedModel: "req", ReportedModel: "actual", InputTokens: int64p(5), OutputTokens: int64p(7), Cost: &costB, Currency: "USD"},
	})
	if reported.CostStatus != costStatusReported || *reported.ProviderCost != "0.00125000" || reported.Currency != "USD" {
		t.Fatalf("reported cost = %+v", reported)
	}
	if reported.Model != "actual" || reported.Attempts != 2 || *reported.InputTokens != 15 || *reported.OutputTokens != 7 || reported.TotalTokens != nil {
		t.Fatalf("usage summary = %+v", reported)
	}

	missing := summarizeCalls([]ai.CallMetadata{{Cost: &costA, Currency: "USD"}, {Characters: int64p(12)}})
	if missing.CostStatus != costStatusMissing || missing.ProviderCost != nil || *missing.Characters != 12 {
		t.Fatalf("partial cost must be missing, got %+v", missing)
	}
	if legacy := summarizeCalls(nil); legacy.CostStatus != costStatusMissing || legacy.Attempts != 0 {
		t.Fatalf("legacy summary = %+v", legacy)
	}
}

func TestSnapshotReportsAllowanceAndGlobalAvailability(t *testing.T) {
	meter, _, now := newTestMeter(Limits{
		FeatureOCR:         {User: 3, Global: 1},
		FeatureExplanation: {User: 5, Global: 50},
		FeatureSpeech:      {User: 0, Global: 50},
	})
	ctx := context.Background()
	if _, _, err := meter.Reserve(ctx, 7, FeatureOCR, 1); err != nil {
		t.Fatalf("reserve: %v", err)
	}

	other, err := meter.Snapshot(ctx, 8)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	ocr := other.Features[0]
	if ocr.Feature != FeatureOCR || ocr.Unit != "page" || ocr.Used != 0 || ocr.Remaining != 3 || ocr.GlobalAvailable {
		t.Fatalf("other user's OCR = %+v; want own allowance and global unavailable", ocr)
	}
	if speech := other.Features[2]; speech.Limit != 0 || speech.Remaining != 0 {
		t.Fatalf("disabled speech = %+v", speech)
	}

	// After the UTC reset the day starts fresh.
	*now = testNow.Add(time.Hour)
	next, err := meter.Snapshot(ctx, 7)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if ocr := next.Features[0]; ocr.Used != 0 || !ocr.GlobalAvailable {
		t.Fatalf("next-day OCR = %+v", ocr)
	}
	if want := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC); !next.ResetAt.Equal(want) {
		t.Fatalf("ResetAt = %v, want %v", next.ResetAt, want)
	}
}

func TestStaleReservationIsRefundedToUserOnly(t *testing.T) {
	meter, store, now := newTestMeter(Limits{FeatureOCR: {User: 1, Global: 10}})
	meter.WithStaleReservationTimeout(FeatureOCR, 15*time.Minute)
	ctx := context.Background()

	_, abandoned, err := meter.Reserve(ctx, 1, FeatureOCR, 1)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	// Still within the cutoff, the page stays charged: a live OCR may be running.
	*now = testNow.Add(14 * time.Minute)
	if _, _, err := meter.Reserve(ctx, 1, FeatureOCR, 1); err == nil {
		t.Fatal("in-flight reservation was refunded before the cutoff")
	}

	*now = testNow.Add(16 * time.Minute)
	snapshot, err := meter.Snapshot(ctx, 1)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if ocr := snapshot.Features[0]; ocr.Remaining != 1 {
		t.Fatalf("snapshot after cutoff = %+v, want the page refunded", ocr)
	}
	if store.Reservations()[0].Status != storage.UsageStatusFailed {
		t.Fatalf("abandoned reservation status = %s", store.Reservations()[0].Status)
	}

	// A late Finish from the abandoned job is a no-op and cannot double refund.
	meter.Finish(ctx, abandoned, &ai.ProviderError{Kind: ai.ErrorKindConfig})
	if _, _, err := meter.Reserve(ctx, 1, FeatureOCR, 1); err != nil {
		t.Fatalf("reserve after refund: %v", err)
	}
	if _, _, err := meter.Reserve(ctx, 1, FeatureOCR, 1); err == nil {
		t.Fatal("late Finish refunded the page twice")
	}
}

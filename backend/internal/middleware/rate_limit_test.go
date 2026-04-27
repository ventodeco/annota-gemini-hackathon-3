package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterRejectsRequestsAfterUserLimit(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute)
	called := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	})
	handler := limiter.Handle(next)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/ai/analyze", nil)
		req = req.WithContext(WithUserID(req.Context(), 42))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/ai/analyze", nil)
	req = req.WithContext(WithUserID(req.Context(), 42))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after limit, got %d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got == "" {
		t.Fatalf("expected Retry-After header")
	}
	if called != 2 {
		t.Fatalf("expected next handler to be called twice, got %d", called)
	}
}

func TestRateLimiterKeepsUsersSeparate(t *testing.T) {
	limiter := NewRateLimiter(1, time.Minute)
	handler := limiter.Handle(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	firstReq := httptest.NewRequest(http.MethodPost, "/v1/ai/speech", nil)
	firstReq = firstReq.WithContext(WithUserID(firstReq.Context(), 1))
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("expected first user request 200, got %d", firstRec.Code)
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/v1/ai/speech", nil)
	secondReq = secondReq.WithContext(WithUserID(secondReq.Context(), 2))
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusOK {
		t.Fatalf("expected second user request 200, got %d", secondRec.Code)
	}
}

func TestRateLimiterSharesQuotaAcrossPathsForUser(t *testing.T) {
	limiter := NewRateLimiter(1, time.Minute)
	handler := limiter.Handle(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	firstReq := httptest.NewRequest(http.MethodPost, "/v1/ai/analyze", nil)
	firstReq = firstReq.WithContext(WithUserID(firstReq.Context(), 42))
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("expected first request 200, got %d", firstRec.Code)
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/v1/ai/speech", nil)
	secondReq = secondReq.WithContext(WithUserID(secondReq.Context(), 42))
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected shared quota to return 429, got %d", secondRec.Code)
	}
}

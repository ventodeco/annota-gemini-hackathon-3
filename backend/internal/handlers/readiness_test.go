package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func readinessRequest(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	return rec
}

func passingCheck(context.Context) error { return nil }

func TestReadinessHandler_ReadyWhenAllChecksPass(t *testing.T) {
	handler := NewReadinessHandler(time.Second,
		ReadinessCheck{Name: "postgres", Check: passingCheck},
		ReadinessCheck{Name: "redis", Check: passingCheck},
	)

	rec := readinessRequest(t, handler)
	if rec.Code != http.StatusOK || rec.Body.String() != "ready" {
		t.Fatalf("got %d %q, want 200 ready", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}

func TestReadinessHandler_NotReadyWhenADependencyFails(t *testing.T) {
	secret := errors.New("dial tcp db.internal:5432: password=hunter2 refused")
	for _, failing := range []string{"postgres", "redis"} {
		t.Run(failing, func(t *testing.T) {
			check := func(name string) ReadinessCheck {
				return ReadinessCheck{Name: name, Check: func(context.Context) error {
					if name == failing {
						return secret
					}
					return nil
				}}
			}
			handler := NewReadinessHandler(time.Second, check("postgres"), check("redis"))

			rec := readinessRequest(t, handler)
			if rec.Code != http.StatusServiceUnavailable || rec.Body.String() != "not ready" {
				t.Fatalf("got %d %q, want 503 not ready", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "hunter2") || strings.Contains(rec.Body.String(), failing) {
				t.Fatal("readiness response must not expose dependency details")
			}
		})
	}
}

func TestReadinessHandler_HangingCheckIsCutOffByDeadline(t *testing.T) {
	handler := NewReadinessHandler(50*time.Millisecond, ReadinessCheck{Name: "postgres", Check: func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}})

	started := time.Now()
	rec := readinessRequest(t, handler)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503", rec.Code)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("readiness took %s, want it bounded by the deadline", elapsed)
	}
}

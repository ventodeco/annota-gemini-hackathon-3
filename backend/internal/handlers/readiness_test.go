package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

func TestReadinessHandler_EnforcesDeadlineWhenCheckIgnoresContext(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	var calls atomic.Int32
	handler := NewReadinessHandler(50*time.Millisecond, ReadinessCheck{Name: "postgres", Check: func(context.Context) error {
		calls.Add(1)
		<-release
		return nil
	}})

	for i := 0; i < 3; i++ {
		started := time.Now()
		rec := readinessRequest(t, handler)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("request %d: got %d, want 503", i+1, rec.Code)
		}
		if elapsed := time.Since(started); elapsed > time.Second {
			t.Fatalf("request %d took %s, want it bounded by the deadline", i+1, elapsed)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("stalled check started %d times, want requests to share one run", got)
	}
}

func TestReadinessHandler_RejectsSuccessAfterDeadline(t *testing.T) {
	handler := NewReadinessHandler(50*time.Millisecond, ReadinessCheck{Name: "redis", Check: func(context.Context) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	}})

	if rec := readinessRequest(t, handler); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503 for a check that succeeds only after the deadline", rec.Code)
	}
}

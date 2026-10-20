package handlers

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/gemini-hackathon/app/internal/logger"
	"github.com/gemini-hackathon/app/internal/middleware"
)

// ReadinessCheck reports whether one runtime dependency can serve requests.
type ReadinessCheck struct {
	Name  string
	Check func(context.Context) error
}

// readinessProbe runs at most one check at a time; concurrent requests share
// the in-flight run, so a check that ignores its context cannot pile up
// goroutines.
type readinessProbe struct {
	check   ReadinessCheck
	timeout time.Duration
	mu      sync.Mutex
	running *readinessRun
}

type readinessRun struct {
	done chan struct{}
	err  error
}

func (p *readinessProbe) start() *readinessRun {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running != nil {
		return p.running
	}

	run := &readinessRun{done: make(chan struct{})}
	p.running = run
	go func() {
		// Detached from any one request because the run may be shared.
		ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
		defer cancel()
		run.err = p.check.Check(ctx)
		p.mu.Lock()
		p.running = nil
		p.mu.Unlock()
		close(run.done)
	}()
	return run
}

var errReadinessDeadline = errors.New("readiness deadline exceeded")

// NewReadinessHandler responds 200 only when every check succeeds within
// timeout, and 503 otherwise. The deadline is enforced by the handler, not by
// the checks. Failure details are logged, never returned.
func NewReadinessHandler(timeout time.Duration, checks ...ReadinessCheck) http.HandlerFunc {
	probes := make([]*readinessProbe, len(checks))
	for i, check := range checks {
		probes[i] = &readinessProbe{check: check, timeout: timeout}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		runs := make([]*readinessRun, len(probes))
		for i, probe := range probes {
			runs[i] = probe.start()
		}

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		for i, run := range runs {
			err := errReadinessDeadline
			select {
			case <-run.done:
				err = run.err
				if err == nil && ctx.Err() != nil {
					err = errReadinessDeadline
				}
			case <-ctx.Done():
			}
			if err != nil {
				logger.GetDefaultLogger().
					WithRequestID(middleware.GetRequestID(r.Context())).
					WithField("dependency", probes[i].check.Name).
					ErrorWithErr(err, "Readiness check failed")
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("not ready"))
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	}
}

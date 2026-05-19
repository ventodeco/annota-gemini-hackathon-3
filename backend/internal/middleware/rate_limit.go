package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gemini-hackathon/app/internal/httputil"
)

type RateLimiter struct {
	limit  int
	window time.Duration
	now    func() time.Time
	mu     sync.Mutex
	hits   map[string][]time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:  limit,
		window: window,
		now:    time.Now,
		hits:   make(map[string][]time.Time),
	}
}

func (l *RateLimiter) Handle(next http.Handler) http.Handler {
	if l.limit <= 0 || l.window <= 0 {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isRateLimitedMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		allowed, retryAfter := l.allow(rateLimitKey(r))
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
			httputil.WriteJSONError(w, http.StatusTooManyRequests, "Usage limit reached. Please try again later.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isRateLimitedMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

func (l *RateLimiter) allow(key string) (bool, time.Duration) {
	now := l.now()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	entries := l.hits[key]
	kept := entries[:0]
	for _, ts := range entries {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}

	if len(kept) >= l.limit {
		oldest := kept[0]
		l.hits[key] = kept
		return false, oldest.Add(l.window).Sub(now)
	}

	kept = append(kept, now)
	l.hits[key] = kept
	return true, 0
}

func rateLimitKey(r *http.Request) string {
	if userID := GetUserID(r.Context()); userID > 0 {
		return fmt.Sprintf("user:%d", userID)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return "ip:" + host
	}
	return "ip:" + r.RemoteAddr
}

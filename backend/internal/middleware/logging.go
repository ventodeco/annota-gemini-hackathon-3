package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gemini-hackathon/app/internal/logger"
)

type requestIDKey string

const RequestIDKey requestIDKey = "request_id"

// RequestIDHeader carries the server-generated request ID on every response so
// users can quote it to support. Client-supplied values are never trusted.
const RequestIDHeader = "X-Request-ID"
const maxLoggedResponseBodyBytes = 4096

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       []byte
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(body []byte) (int, error) {
	remaining := maxLoggedResponseBodyBytes - len(rw.body)
	if remaining > 0 {
		if len(body) > remaining {
			rw.body = append(rw.body, body[:remaining]...)
		} else {
			rw.body = append(rw.body, body...)
		}
	}
	return rw.ResponseWriter.Write(body)
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := generateRequestID()

		w.Header().Set(RequestIDHeader, requestID)
		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		loggedUser := &requestUser{}
		ctx = context.WithValue(ctx, requestUserKey, loggedUser)
		r = r.WithContext(ctx)

		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(ww, r)

		duration := time.Since(start)

		reqLogger := logger.GetDefaultLogger().WithRequestID(requestID)

		if uid := loggedUser.id.Load(); uid != 0 {
			reqLogger = reqLogger.WithUserID(uid)
		}

		reqLogger.RequestLog(r.Method, r.URL.Path, ww.statusCode, duration)

		if ww.statusCode >= 400 {
			reqLogger.Warnf("Request failed: %s %s - Status: %d - Duration: %v - Body: %s",
				r.Method, r.URL.Path, ww.statusCode, duration, string(ww.body))
		}
	})
}

type requestUserKeyType struct{}

var requestUserKey requestUserKeyType

// requestUser carries the authenticated account back out to the request log:
// auth runs inside LoggingMiddleware and passes a derived request onward, so
// the outer request's context never sees the user ID it adds.
type requestUser struct {
	id atomic.Int64
}

// recordRequestUser reports the authenticated account to this request's log.
func recordRequestUser(ctx context.Context, userID int64) {
	if holder, ok := ctx.Value(requestUserKey).(*requestUser); ok {
		holder.id.Store(userID)
	}
}

func generateRequestID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gemini-hackathon/app/internal/logger"
)

type requestIDKey string

const RequestIDKey requestIDKey = "request_id"
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

		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		r = r.WithContext(ctx)

		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(ww, r)

		duration := time.Since(start)

		reqLogger := logger.GetDefaultLogger().WithRequestID(requestID)

		if userID := r.Context().Value(userIDKey); userID != nil {
			if uid, ok := userID.(int64); ok {
				reqLogger = reqLogger.WithUserID(uid)
			}
		}

		reqLogger.RequestLog(r.Method, r.URL.Path, ww.statusCode, duration)

		if ww.statusCode >= 400 {
			reqLogger.Warnf("Request failed: %s %s - Status: %d - Duration: %v - Body: %s",
				r.Method, r.URL.Path, ww.statusCode, duration, string(ww.body))
		}
	})
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

package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/logger"
)

func TestResponseWriterCapturesMultipleWritesWithCap(t *testing.T) {
	rec := httptest.NewRecorder()
	writer := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	if _, err := writer.Write([]byte("first ")); err != nil {
		t.Fatalf("first Write returned error: %v", err)
	}
	if _, err := writer.Write([]byte("second")); err != nil {
		t.Fatalf("second Write returned error: %v", err)
	}

	if got := string(writer.body); got != "first second" {
		t.Fatalf("captured body = %q; want %q", got, "first second")
	}
}

func TestLoggingMiddlewareExposesLoggedRequestID(t *testing.T) {
	logFile, err := os.CreateTemp(t.TempDir(), "log")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	logger.SetOutput(logFile)
	t.Cleanup(func() { logger.SetOutput(os.Stdout) })

	var handlerID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusBadGateway)
	})
	cors := NewCORSMiddleware(&config.Config{AllowedOrigins: "http://localhost:5173"})
	req := httptest.NewRequest(http.MethodPost, "/v1/ai/analyze", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set(RequestIDHeader, "client-chosen")
	rec := httptest.NewRecorder()

	LoggingMiddleware(cors.Handle(next)).ServeHTTP(rec, req)

	got := rec.Header().Get(RequestIDHeader)
	if got == "" || got == "client-chosen" || got != handlerID {
		t.Fatalf("X-Request-ID = %q, handler saw %q; want the server-generated ID", got, handlerID)
	}
	if exposed := rec.Header().Get("Access-Control-Expose-Headers"); exposed != RequestIDHeader {
		t.Fatalf("Access-Control-Expose-Headers = %q", exposed)
	}

	logged, err := os.ReadFile(logFile.Name())
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	var entry struct {
		RequestID  string `json:"request_id"`
		DurationMS *int64 `json:"duration_ms"`
	}
	firstLine, _, _ := strings.Cut(string(logged), "\n")
	if err := json.Unmarshal([]byte(firstLine), &entry); err != nil {
		t.Fatalf("decode request log %q: %v", firstLine, err)
	}
	if entry.RequestID != got || entry.DurationMS == nil {
		t.Fatalf("request log = %+v; want request_id %q and duration_ms", entry, got)
	}
}

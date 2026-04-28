package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
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

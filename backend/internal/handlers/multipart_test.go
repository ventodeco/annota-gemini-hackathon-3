package handlers

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadFormFileRejectsRequestBodyOverUploadLimit(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "large.pdf")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := part.Write([]byte(strings.Repeat("x", int(maxMultipartBodyBytes(8))))); err != nil {
		t.Fatalf("part.Write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	_, _, err = readFormFile(rec, req, 8, "file")
	if !errors.Is(err, ErrUploadTooLarge) {
		t.Fatalf("expected ErrUploadTooLarge, got %v", err)
	}
}

package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testIndexHTML = "<!doctype html><title>app</title>"

func writeDist(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dist := filepath.Join(root, "dist")
	files := map[string]string{
		"index.html":           testIndexHTML,
		"assets/app-1234.js":   "console.log('app')",
		"assets/app-1234.css":  "body{margin:0}",
		"manifest.webmanifest": `{"name":"Annota"}`,
	}
	for name, content := range files {
		target := filepath.Join(dist, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("outside dist"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	return dist
}

func serveFrontend(t *testing.T, handler http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.URL.Path = target
	handler.ServeHTTP(rec, req)
	return rec
}

func TestFrontendHandler_ServesExistingAssets(t *testing.T) {
	handler := NewFrontendHandler(writeDist(t))
	tests := []struct {
		path, body, contentType string
	}{
		{"/assets/app-1234.js", "console.log('app')", "javascript"},
		{"/assets/app-1234.css", "body{margin:0}", "text/css"},
		{"/manifest.webmanifest", `{"name":"Annota"}`, ""},
	}
	for _, tt := range tests {
		rec := serveFrontend(t, handler, tt.path)
		if rec.Code != http.StatusOK || rec.Body.String() != tt.body {
			t.Fatalf("%s: got %d %q, want 200 %q", tt.path, rec.Code, rec.Body.String(), tt.body)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), tt.contentType) {
			t.Fatalf("%s: Content-Type = %q, want %q", tt.path, rec.Header().Get("Content-Type"), tt.contentType)
		}
	}
}

func TestFrontendHandler_ServesEntryDocumentForClientRoutes(t *testing.T) {
	handler := NewFrontendHandler(writeDist(t))
	for _, target := range []string{"/", "/login", "/documents/42/read"} {
		rec := serveFrontend(t, handler, target)
		if rec.Code != http.StatusOK || rec.Body.String() != testIndexHTML {
			t.Fatalf("%s: got %d %q, want entry document", target, rec.Code, rec.Body.String())
		}
	}
}

func TestFrontendHandler_NotFoundForMissingAssetsAndReservedPaths(t *testing.T) {
	handler := NewFrontendHandler(writeDist(t))
	for _, target := range []string{"/assets/missing-chunk.js", "/icon.png", "/v1/unknown", "/healthz/extra", "/readyz/extra", "/../secret.txt"} {
		rec := serveFrontend(t, handler, target)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: got %d, want 404", target, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "outside dist") || strings.Contains(rec.Body.String(), "<!doctype html>") {
			t.Fatalf("%s: unexpected body %q", target, rec.Body.String())
		}
	}
}

func TestFrontendHandler_UnavailableWithoutBuild(t *testing.T) {
	rec := serveFrontend(t, NewFrontendHandler(t.TempDir()), "/login")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503", rec.Code)
	}
}

package handlers

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var reservedFrontendPrefixes = []string{"/v1", "/healthz", "/readyz"}

// NewFrontendHandler serves a built single-page app from distDir. Existing
// files are served as is, missing paths with a file extension return 404, and
// other paths return index.html for client-side routing.
func NewFrontendHandler(distDir string) http.Handler {
	files := http.FileServer(http.Dir(distDir))
	index := filepath.Join(distDir, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlPath := path.Clean("/" + r.URL.Path)
		for _, prefix := range reservedFrontendPrefixes {
			if urlPath == prefix || strings.HasPrefix(urlPath, prefix+"/") {
				http.NotFound(w, r)
				return
			}
		}

		if _, err := os.Stat(index); err != nil {
			http.Error(w, "Frontend not built. Run: cd web && bun run build", http.StatusServiceUnavailable)
			return
		}

		info, err := os.Stat(filepath.Join(distDir, filepath.FromSlash(urlPath)))
		if err == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}
		if path.Ext(urlPath) != "" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, index)
	})
}

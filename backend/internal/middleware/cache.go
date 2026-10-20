package middleware

import "net/http"

// NoStore marks every response as uncacheable. Authenticated API responses
// are keyed only by URL in browser and proxy caches, so a stored response
// could be shown to the next account on the same browser or outlive deletion.
func NoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

package httputil

import (
	"context"
	"net/http"
	"strconv"
)

// ParsePagination extracts page and size from query parameters.
// Invalid or non-positive values are intentionally coerced to safe defaults,
// and size is capped at 100 to protect list endpoints.
func ParsePagination(r *http.Request, defaultSize int) (page, size int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	size, _ = strconv.Atoi(r.URL.Query().Get("size"))
	if size < 1 {
		size = defaultSize
	}
	if size > 100 {
		size = 100
	}

	return page, size
}

// RequireUserID extracts the user ID using the provided function and writes
// a 401 JSON error if the user is not authenticated. Returns the userID and
// true if authenticated, or 0 and false if not.
func RequireUserID(w http.ResponseWriter, r *http.Request, getUserID func(context.Context) int64) (int64, bool) {
	userID := getUserID(r.Context())
	if userID == 0 {
		WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return 0, false
	}
	return userID, true
}

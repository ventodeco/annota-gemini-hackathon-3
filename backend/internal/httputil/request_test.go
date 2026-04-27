package httputil

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		defaultSize  int
		expectedPage int
		expectedSize int
	}{
		{
			name:         "no params uses defaults",
			query:        "",
			defaultSize:  20,
			expectedPage: 1,
			expectedSize: 20,
		},
		{
			name:         "valid page and size",
			query:        "page=3&size=10",
			defaultSize:  20,
			expectedPage: 3,
			expectedSize: 10,
		},
		{
			name:         "zero page defaults to 1",
			query:        "page=0&size=10",
			defaultSize:  20,
			expectedPage: 1,
			expectedSize: 10,
		},
		{
			name:         "negative page defaults to 1",
			query:        "page=-5&size=10",
			defaultSize:  20,
			expectedPage: 1,
			expectedSize: 10,
		},
		{
			name:         "zero size uses default",
			query:        "page=1&size=0",
			defaultSize:  15,
			expectedPage: 1,
			expectedSize: 15,
		},
		{
			name:         "negative size uses default",
			query:        "page=1&size=-3",
			defaultSize:  25,
			expectedPage: 1,
			expectedSize: 25,
		},
		{
			name:         "size exceeding 100 capped to 100",
			query:        "page=1&size=200",
			defaultSize:  20,
			expectedPage: 1,
			expectedSize: 100,
		},
		{
			name:         "non-numeric page defaults to 1",
			query:        "page=abc&size=10",
			defaultSize:  20,
			expectedPage: 1,
			expectedSize: 10,
		},
		{
			name:         "non-numeric size uses default",
			query:        "page=1&size=xyz",
			defaultSize:  20,
			expectedPage: 1,
			expectedSize: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/?"+tt.query, nil)
			page, size := ParsePagination(r, tt.defaultSize)

			if page != tt.expectedPage {
				t.Errorf("page = %d; want %d", page, tt.expectedPage)
			}
			if size != tt.expectedSize {
				t.Errorf("size = %d; want %d", size, tt.expectedSize)
			}
		})
	}
}

type contextKey string

const testUserIDKey contextKey = "userID"

func TestRequireUserID(t *testing.T) {
	t.Run("returns userID when present in context", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := context.WithValue(r.Context(), testUserIDKey, int64(42))
		r = r.WithContext(ctx)
		w := httptest.NewRecorder()

		getUserID := func(ctx context.Context) int64 {
			if id, ok := ctx.Value(testUserIDKey).(int64); ok {
				return id
			}
			return 0
		}

		userID, ok := RequireUserID(w, r, getUserID)

		if !ok {
			t.Fatal("expected ok=true, got false")
		}
		if userID != 42 {
			t.Errorf("userID = %d; want 42", userID)
		}
		if w.Code != http.StatusOK {
			t.Errorf("status = %d; want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("returns error when userID is 0", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		getUserID := func(ctx context.Context) int64 {
			return 0
		}

		userID, ok := RequireUserID(w, r, getUserID)

		if ok {
			t.Fatal("expected ok=false, got true")
		}
		if userID != 0 {
			t.Errorf("userID = %d; want 0", userID)
		}
		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d; want %d", w.Code, http.StatusUnauthorized)
		}

		// Verify JSON error response
		var resp ErrorResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode error response: %v", err)
		}
		if resp.Message != "Unauthorized" {
			t.Errorf("message = %q; want %q", resp.Message, "Unauthorized")
		}
	})
}

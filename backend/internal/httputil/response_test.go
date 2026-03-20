package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSONError(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		message        string
		expectedError  string
		expectedMsg    string
	}{
		{
			name:          "bad request",
			statusCode:    http.StatusBadRequest,
			message:       "Invalid input",
			expectedError: "Bad Request",
			expectedMsg:   "Invalid input",
		},
		{
			name:          "unauthorized",
			statusCode:    http.StatusUnauthorized,
			message:       "Unauthorized",
			expectedError: "Unauthorized",
			expectedMsg:   "Unauthorized",
		},
		{
			name:          "internal server error",
			statusCode:    http.StatusInternalServerError,
			message:       "Something went wrong",
			expectedError: "Internal Server Error",
			expectedMsg:   "Something went wrong",
		},
		{
			name:          "not found",
			statusCode:    http.StatusNotFound,
			message:       "User not found",
			expectedError: "Not Found",
			expectedMsg:   "User not found",
		},
		{
			name:          "method not allowed",
			statusCode:    http.StatusMethodNotAllowed,
			message:       "Method not allowed",
			expectedError: "Method Not Allowed",
			expectedMsg:   "Method not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			WriteJSONError(w, tt.statusCode, tt.message)

			// Check status code
			if w.Code != tt.statusCode {
				t.Errorf("status code = %d; want %d", w.Code, tt.statusCode)
			}

			// Check Content-Type header
			ct := w.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("Content-Type = %q; want %q", ct, "application/json")
			}

			// Check body is valid JSON with correct fields
			var resp ErrorResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}

			if resp.Error != tt.expectedError {
				t.Errorf("Error = %q; want %q", resp.Error, tt.expectedError)
			}
			if resp.Message != tt.expectedMsg {
				t.Errorf("Message = %q; want %q", resp.Message, tt.expectedMsg)
			}
		})
	}
}

func TestWriteJSON(t *testing.T) {
	type testPayload struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	tests := []struct {
		name       string
		statusCode int
		data       interface{}
	}{
		{
			name:       "ok with struct",
			statusCode: http.StatusOK,
			data:       testPayload{Name: "test", Value: 42},
		},
		{
			name:       "created with map",
			statusCode: http.StatusCreated,
			data:       map[string]string{"id": "123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			WriteJSON(w, tt.statusCode, tt.data)

			if w.Code != tt.statusCode {
				t.Errorf("status code = %d; want %d", w.Code, tt.statusCode)
			}

			ct := w.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("Content-Type = %q; want %q", ct, "application/json")
			}

			// Verify it's valid JSON
			var raw json.RawMessage
			if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
				t.Fatalf("response body is not valid JSON: %v", err)
			}
		})
	}
}

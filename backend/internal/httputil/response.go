package httputil

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse is the standard JSON error response format for all API endpoints.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	// Code is an optional stable machine-readable reason, e.g. rate_limited.
	Code string `json:"code,omitempty"`
}

// WriteJSONError writes a JSON error response with the given status code and message.
func WriteJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
	})
}

// WriteJSONErrorCode writes a JSON error response with a stable error code.
func WriteJSONErrorCode(w http.ResponseWriter, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
		Code:    code,
	})
}

// WriteJSON writes a JSON response with the given status code and data.
func WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

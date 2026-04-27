package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gemini-hackathon/app/internal/httputil"
	"github.com/gemini-hackathon/app/internal/logger"
	"github.com/gemini-hackathon/app/internal/middleware"
)

type AnalyticsHandlers struct{}

type AnalyticsEventRequest struct {
	Name       string         `json:"name"`
	Properties map[string]any `json:"properties,omitempty"`
}

func NewAnalyticsHandlers() *AnalyticsHandlers {
	return &AnalyticsHandlers{}
}

func (h *AnalyticsHandlers) EventsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req AnalyticsEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "name is required")
		return
	}

	logger.GetDefaultLogger().
		WithRequestID(middleware.GetRequestID(r.Context())).
		WithUserID(userID).
		WithFields(map[string]any{
			"event":      req.Name,
			"properties": req.Properties,
		}).
		Info("analytics_event")

	w.WriteHeader(http.StatusAccepted)
}

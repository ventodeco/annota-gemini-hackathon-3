package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gemini-hackathon/app/internal/httputil"
	"github.com/gemini-hackathon/app/internal/logger"
	"github.com/gemini-hackathon/app/internal/middleware"
)

const maxAnalyticsPayloadBytes int64 = 2048

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
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxAnalyticsPayloadBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "name is required")
		return
	}
	properties, err := allowedAnalyticsProperties(req)
	if err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	logger.GetDefaultLogger().
		WithRequestID(middleware.GetRequestID(r.Context())).
		WithUserID(userID).
		WithFields(map[string]any{
			"event":      req.Name,
			"properties": properties,
		}).
		Info("analytics_event")

	w.WriteHeader(http.StatusAccepted)
}

func allowedAnalyticsProperties(req AnalyticsEventRequest) (map[string]any, error) {
	switch req.Name {
	case "first_annotation", "reader_activation":
	default:
		return nil, errors.New("unknown event")
	}

	properties := make(map[string]any, len(req.Properties))
	for key, value := range req.Properties {
		switch key {
		case "source":
			source, ok := value.(string)
			if !ok {
				return nil, errors.New("invalid property")
			}
			switch source {
			case "image", "pdf", "camera", "gallery":
				properties[key] = source
			default:
				return nil, errors.New("invalid property")
			}
		default:
			return nil, errors.New("unknown property")
		}
	}
	return properties, nil
}

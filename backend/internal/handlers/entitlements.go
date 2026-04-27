package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/httputil"
	"github.com/gemini-hackathon/app/internal/middleware"
)

type EntitlementHandlers struct {
	config *config.Config
}

type EntitlementLimits struct {
	AIRequestsPerWindow int   `json:"aiRequestsPerWindow"`
	AIWindowSeconds     int   `json:"aiWindowSeconds"`
	MaxUploadSize       int64 `json:"maxUploadSize"`
}

type EntitlementResponse struct {
	Plan   string            `json:"plan"`
	Limits EntitlementLimits `json:"limits"`
}

func NewEntitlementHandlers(cfg *config.Config) *EntitlementHandlers {
	return &EntitlementHandlers{config: cfg}
}

func (h *EntitlementHandlers) MeAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	if middleware.GetUserID(r.Context()) == 0 {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(EntitlementResponse{
		Plan: "free",
		Limits: EntitlementLimits{
			AIRequestsPerWindow: h.config.AIRateLimit,
			AIWindowSeconds:     h.config.AIRateLimitWindowSeconds,
			MaxUploadSize:       h.config.MaxUploadSize,
		},
	})
}

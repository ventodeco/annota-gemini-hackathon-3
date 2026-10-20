package handlers

import (
	"net/http"
	"time"

	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/httputil"
	"github.com/gemini-hackathon/app/internal/logger"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/usage"
)

type EntitlementHandlers struct {
	config *config.Config
	meter  *usage.Meter
}

type EntitlementLimits struct {
	AIRequestsPerWindow int   `json:"aiRequestsPerWindow"`
	AIWindowSeconds     int   `json:"aiWindowSeconds"`
	MaxUploadSize       int64 `json:"maxUploadSize"`
}

// EntitlementFeatureUsage is the caller's daily allowance for one feature.
// Global budget totals are reduced to availability.
type EntitlementFeatureUsage struct {
	Feature         string `json:"feature"`
	Unit            string `json:"unit"`
	Limit           int64  `json:"limit"`
	Used            int64  `json:"used"`
	Remaining       int64  `json:"remaining"`
	MaxPerRequest   *int64 `json:"maxPerRequest,omitempty"`
	GlobalAvailable bool   `json:"globalAvailable"`
}

type EntitlementUsage struct {
	Period   string                    `json:"period"`
	Timezone string                    `json:"timezone"`
	ResetAt  string                    `json:"resetAt"`
	Features []EntitlementFeatureUsage `json:"features"`
}

type EntitlementResponse struct {
	Plan   string            `json:"plan"`
	Limits EntitlementLimits `json:"limits"`
	Usage  EntitlementUsage  `json:"usage"`
}

func NewEntitlementHandlers(cfg *config.Config, meter *usage.Meter) *EntitlementHandlers {
	return &EntitlementHandlers{config: cfg, meter: meter}
}

func (h *EntitlementHandlers) MeAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	snapshot, err := h.meter.Snapshot(r.Context(), userID)
	if err != nil {
		logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context())).WithUserID(userID).
			ErrorWithErr(err, "Failed to read usage allowance")
		httputil.WriteJSONErrorCode(w, http.StatusServiceUnavailable, usageCodeUnavailable, "Your usage allowance could not be loaded. Please try again shortly.")
		return
	}

	features := make([]EntitlementFeatureUsage, 0, len(snapshot.Features))
	for _, f := range snapshot.Features {
		item := EntitlementFeatureUsage{
			Feature:         string(f.Feature),
			Unit:            f.Unit,
			Limit:           f.Limit,
			Used:            f.Used,
			Remaining:       f.Remaining,
			GlobalAvailable: f.GlobalAvailable,
		}
		if perRequest := h.meter.Limit(f.Feature).PerRequest; perRequest > 0 {
			item.MaxPerRequest = &perRequest
		}
		features = append(features, item)
	}

	httputil.WriteJSON(w, http.StatusOK, EntitlementResponse{
		Plan: "free",
		Limits: EntitlementLimits{
			AIRequestsPerWindow: h.config.AIRateLimit,
			AIWindowSeconds:     h.config.AIRateLimitWindowSeconds,
			MaxUploadSize:       h.config.MaxUploadSize,
		},
		Usage: EntitlementUsage{
			Period:   "daily",
			Timezone: "UTC",
			ResetAt:  snapshot.ResetAt.UTC().Format(time.RFC3339),
			Features: features,
		},
	})
}

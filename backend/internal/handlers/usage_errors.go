package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gemini-hackathon/app/internal/logger"
	"github.com/gemini-hackathon/app/internal/usage"
)

const (
	usageCodeQuotaExceeded   = "usage_quota_exceeded"
	usageCodeBudgetExhausted = "usage_budget_exhausted"
	usageCodeTooLarge        = "usage_request_too_large"
	usageCodeUnavailable     = "usage_unavailable"
)

// UsageErrorResponse extends the standard error shape with allowance details.
// Global budget errors carry only the feature and reset time.
type UsageErrorResponse struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	Code      string `json:"code"`
	Feature   string `json:"feature,omitempty"`
	Limit     *int64 `json:"limit,omitempty"`
	Used      *int64 `json:"used,omitempty"`
	Remaining *int64 `json:"remaining,omitempty"`
	Max       *int64 `json:"max,omitempty"`
	ResetAt   string `json:"resetAt,omitempty"`
}

var usageFeatureLabels = map[usage.Feature]string{
	usage.FeatureOCR:         "image scan",
	usage.FeatureExplanation: "explanation",
	usage.FeatureSpeech:      "speech",
}

// writeUsageError maps an admission failure to an actionable response. It
// never returns 401, so a budget rejection cannot end the user's session.
func writeUsageError(w http.ResponseWriter, err error, log *logger.Logger) {
	var quotaErr *usage.QuotaError
	var tooLarge *usage.TooLargeError
	switch {
	case errors.As(err, &quotaErr):
		label := usageFeatureLabels[quotaErr.Feature]
		body := UsageErrorResponse{
			Error:   http.StatusText(http.StatusTooManyRequests),
			Feature: string(quotaErr.Feature),
			ResetAt: quotaErr.ResetAt.UTC().Format(time.RFC3339),
		}
		if quotaErr.Global {
			body.Code = usageCodeBudgetExhausted
			body.Message = fmt.Sprintf("Today's beta capacity for %ss is used up. Please try again after the daily reset.", label)
		} else {
			remaining := quotaErr.Remaining()
			body.Code = usageCodeQuotaExceeded
			body.Message = fmt.Sprintf("You have used today's %s allowance. It resets daily at 00:00 UTC.", label)
			body.Limit, body.Used, body.Remaining = &quotaErr.Limit, &quotaErr.Used, &remaining
		}
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds(quotaErr.ResetAt)))
		writeUsageJSON(w, http.StatusTooManyRequests, body)
	case errors.As(err, &tooLarge):
		label := usageFeatureLabels[tooLarge.Feature]
		writeUsageJSON(w, http.StatusRequestEntityTooLarge, UsageErrorResponse{
			Error:   http.StatusText(http.StatusRequestEntityTooLarge),
			Code:    usageCodeTooLarge,
			Feature: string(tooLarge.Feature),
			Max:     &tooLarge.Max,
			Message: fmt.Sprintf("This %s request is too large. The limit is %d %ss per request.", label, tooLarge.Max, tooLarge.Feature.Unit()),
		})
	default:
		log.ErrorWithErr(err, "Usage admission unavailable")
		writeUsageJSON(w, http.StatusServiceUnavailable, UsageErrorResponse{
			Error:   http.StatusText(http.StatusServiceUnavailable),
			Code:    usageCodeUnavailable,
			Message: "Your usage allowance could not be checked. Please try again shortly.",
		})
	}
}

func retryAfterSeconds(resetAt time.Time) int {
	return max(int(math.Ceil(time.Until(resetAt).Seconds())), 1)
}

func writeUsageJSON(w http.ResponseWriter, status int, body UsageErrorResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

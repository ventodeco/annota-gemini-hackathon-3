package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type RoutedClientOptions struct {
	HTTPClient *http.Client
	Observe    func(context.Context, CallMetadata)
}

// CallMetadata deliberately has no free-form error or request/response payload.
// Nil accounting fields mean unknown, never a free request.
type CallMetadata struct {
	Task           string       `json:"task"`
	Profile        string       `json:"profile"`
	Gateway        string       `json:"gateway"`
	Region         string       `json:"region"`
	RequestedModel string       `json:"requested_model"`
	ReportedModel  string       `json:"reported_model,omitempty"`
	RequestID      string       `json:"provider_request_id,omitempty"`
	Attempt        int          `json:"attempt"`
	DurationMS     int64        `json:"duration_ms"`
	StatusCode     int          `json:"status_code"`
	ErrorKind      ErrorKind    `json:"error_kind,omitempty"`
	InputTokens    *int64       `json:"input_tokens"`
	OutputTokens   *int64       `json:"output_tokens"`
	TotalTokens    *int64       `json:"total_tokens"`
	Characters     *int64       `json:"characters"`
	Cost           *json.Number `json:"cost"`
	Currency       string       `json:"currency,omitempty"`
}

type taskGateway struct {
	route      TaskRoute
	profile    GatewayProfile
	model      ModelCapabilities
	task       string
	httpClient *http.Client
	observe    func(context.Context, CallMetadata)
}

func newTaskGateway(cfg RoutingConfig, route TaskRoute, task string, opts RoutedClientOptions) *taskGateway {
	// Copy the client so redirect policy is local; preserve injected transports/timeouts.
	client := http.Client{}
	if opts.HTTPClient != nil {
		client = *opts.HTTPClient
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	p := cfg.Profiles[route.Profile]
	model := p.Models[route.Model]
	model.ReportedModelAliases = slices.Clone(model.ReportedModelAliases)
	model.JapaneseVoices = slices.Clone(model.JapaneseVoices)
	model.AudioFormats = slices.Clone(model.AudioFormats)
	p.Models = nil
	return &taskGateway{route: route, profile: p, model: model, task: task, httpClient: &client, observe: opts.Observe}
}

func (g *taskGateway) failure(kind ErrorKind, status int, message string, cause error) error {
	return &ProviderError{Provider: g.profile.Kind, Kind: kind, StatusCode: status, Message: message, Cause: cause}
}

func statusKind(status int) ErrorKind {
	switch status {
	case 401, 403:
		return ErrorKindAuth
	case 402:
		return ErrorKindQuota
	case 429:
		return ErrorKindRateLimit
	default:
		return ErrorKindUpstream
	}
}

func (g *taskGateway) safeID(id string) string {
	if len(id) > 128 || strings.Contains(id, g.profile.APIKey) {
		return ""
	}
	for _, r := range id {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("-_.:/", r) {
			return ""
		}
	}
	return id
}

type responseParser func([]byte, http.Header, *CallMetadata) error

func (g *taskGateway) call(ctx context.Context, path string, body any, limit int64, parse responseParser) error {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(g.profile.TimeoutSeconds)*time.Second)
	defer cancel()
	payload, err := json.Marshal(body)
	if err != nil {
		return g.failure(ErrorKindConfig, 0, "cannot encode AI request", nil)
	}
	for attempt := 1; attempt <= g.profile.MaxAttempts; attempt++ {
		if ctx.Err() != nil {
			return g.failure(ErrorKindUpstream, 0, "AI request cancelled or deadline exceeded", ctx.Err())
		}
		meta := CallMetadata{Task: g.task, Profile: g.route.Profile, Gateway: g.profile.Kind, Region: g.profile.Region, RequestedModel: g.route.Model, Attempt: attempt}
		started := time.Now()
		retryAfter := ""
		callErr := func() error {
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint(g.profile.BaseURL, path), bytes.NewReader(payload))
			if err != nil {
				return g.failure(ErrorKindConfig, 0, "invalid AI endpoint", nil)
			}
			req.Header.Set("Authorization", "Bearer "+g.profile.APIKey)
			req.Header.Set("Content-Type", "application/json")
			resp, err := g.httpClient.Do(req)
			if err != nil {
				cause := err
				if ctx.Err() != nil {
					cause = ctx.Err()
				}
				return g.failure(ErrorKindUpstream, 0, "AI transport failed", cause)
			}
			defer resp.Body.Close()
			meta.StatusCode = resp.StatusCode
			meta.RequestID = g.safeID(resp.Header.Get("X-Request-ID"))
			if meta.RequestID == "" {
				meta.RequestID = g.safeID(resp.Header.Get("X-Generation-ID"))
			}
			retryAfter = resp.Header.Get("Retry-After")
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return g.failure(statusKind(resp.StatusCode), resp.StatusCode, "AI gateway rejected the request", nil)
			}
			data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
			if err != nil {
				if ctx.Err() != nil {
					return g.failure(ErrorKindUpstream, resp.StatusCode, "AI response interrupted", ctx.Err())
				}
				return g.failure(ErrorKindUpstream, resp.StatusCode, "cannot read AI response", err)
			}
			if int64(len(data)) > limit {
				return g.failure(ErrorKindUpstream, resp.StatusCode, "AI response exceeds size limit", nil)
			}
			return parse(data, resp.Header, &meta)
		}()
		meta.DurationMS = time.Since(started).Milliseconds()
		if callErr != nil {
			if e, ok := callErr.(*ProviderError); ok {
				meta.ErrorKind = e.Kind
			} else {
				meta.ErrorKind = ErrorKindUpstream
				callErr = g.failure(ErrorKindUpstream, meta.StatusCode, "invalid AI response", nil)
			}
		}
		if g.observe != nil {
			g.observe(ctx, meta)
		}
		if callErr == nil {
			return nil
		}
		if attempt == g.profile.MaxAttempts || (meta.StatusCode != 429 && meta.StatusCode != 503) {
			return callErr
		}
		delay := retryDelay(retryAfter, time.Now())
		if deadline, ok := ctx.Deadline(); ok && delay >= time.Until(deadline) {
			return callErr
		}
		if !sleepWithContext(ctx, delay) {
			return g.failure(ErrorKindUpstream, meta.StatusCode, "AI retry cancelled", ctx.Err())
		}
	}
	return fmt.Errorf("AI attempts exhausted")
}

func retryDelay(raw string, now time.Time) time.Duration {
	// Saturate long valid numeric delays so they suppress retries rather than
	// wrapping or being treated as missing headers. The operation has its own deadline.
	const maxDelay = time.Duration(1<<63 - 1)
	if seconds, err := strconv.ParseUint(raw, 10, 64); err == nil || errors.Is(err, strconv.ErrRange) {
		if seconds > uint64(maxDelay/time.Second) {
			return maxDelay
		}
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(raw); err == nil {
		if when.After(now) {
			return when.Sub(now)
		}
		return 0
	}
	return 500 * time.Millisecond
}

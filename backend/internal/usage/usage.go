// Package usage admits paid AI work against persisted daily allowances: a
// per-user quota and a global beta budget for each feature. See
// docs/usage-quotas.md for the charging and settlement policy.
package usage

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gemini-hackathon/app/internal/ai"
	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/logger"
	"github.com/gemini-hackathon/app/internal/storage"
)

type Feature string

const (
	FeatureOCR         Feature = "ocr"
	FeatureExplanation Feature = "explanation"
	FeatureSpeech      Feature = "speech"
)

// Features lists every metered feature in display order.
var Features = []Feature{FeatureOCR, FeatureExplanation, FeatureSpeech}

// Unit names the quantity a feature is counted in.
func (f Feature) Unit() string {
	switch f {
	case FeatureOCR:
		return "page"
	case FeatureSpeech:
		return "character"
	default:
		return "request"
	}
}

// Limit is a daily allowance for one feature. Zero disables the feature.
// PerRequest, when positive, caps a single request.
type Limit struct {
	User       int64
	Global     int64
	PerRequest int64
}

type Limits map[Feature]Limit

func LimitsFromConfig(cfg *config.Config) Limits {
	return Limits{
		FeatureOCR:         {User: int64(cfg.UsageOCRPagesPerDay), Global: int64(cfg.UsageGlobalOCRPagesPerDay)},
		FeatureExplanation: {User: int64(cfg.UsageExplanationsPerDay), Global: int64(cfg.UsageGlobalExplanationsPerDay)},
		FeatureSpeech: {
			User:       int64(cfg.UsageSpeechCharsPerDay),
			Global:     int64(cfg.UsageGlobalSpeechCharsPerDay),
			PerRequest: int64(cfg.UsageSpeechMaxCharsPerRequest),
		},
	}
}

// QuotaError rejects an admission. Global errors must not expose global totals
// to users; Limit and Used describe the user's own allowance.
type QuotaError struct {
	Feature Feature
	Global  bool
	Limit   int64
	Used    int64
	ResetAt time.Time
}

func (e *QuotaError) Error() string {
	if e.Global {
		return fmt.Sprintf("global %s budget exhausted", e.Feature)
	}
	return fmt.Sprintf("%s quota exceeded", e.Feature)
}

// Remaining is clamped at zero when an operator lowers a limit mid-day.
func (e *QuotaError) Remaining() int64 {
	return max(e.Limit-e.Used, 0)
}

// TooLargeError means one request needs more units than any day allows, so
// waiting for the reset would not help.
type TooLargeError struct {
	Feature Feature
	Max     int64
}

func (e *TooLargeError) Error() string {
	return fmt.Sprintf("%s request exceeds %d %ss", e.Feature, e.Max, e.Feature.Unit())
}

// DayStart returns the UTC day containing t; ResetAt is the next UTC midnight.
func DayStart(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func ResetAt(t time.Time) time.Time {
	return DayStart(t).AddDate(0, 0, 1)
}

const settleTimeout = 5 * time.Second

type Meter struct {
	store  storage.UsageStore
	limits Limits
	now    func() time.Time
	// staleAfter marks reservations of a feature as abandoned once they are
	// older than any live process could take to settle them.
	staleAfter map[Feature]time.Duration
}

func NewMeter(store storage.UsageStore, limits Limits) *Meter {
	return NewMeterWithClock(store, limits, time.Now)
}

func NewMeterWithClock(store storage.UsageStore, limits Limits, now func() time.Time) *Meter {
	return &Meter{store: store, limits: limits, now: now}
}

// WithStaleReservationTimeout makes the meter settle the caller's
// reservations of feature that are still reserved after timeout as failed:
// the user is refunded and the global charge is kept, because the provider
// may have billed. It covers work whose process died before settling (e.g.
// OCR across a restart). timeout must exceed the longest time a live process
// can take to settle. Configure it before the meter is used.
func (m *Meter) WithStaleReservationTimeout(feature Feature, timeout time.Duration) *Meter {
	if m.staleAfter == nil {
		m.staleAfter = make(map[Feature]time.Duration)
	}
	m.staleAfter[feature] = timeout
	return m
}

// expireStale settles the user's abandoned reservations before their usage is
// read or charged. Failure only delays a refund, so it never blocks the caller.
func (m *Meter) expireStale(ctx context.Context, userID int64, features ...Feature) {
	now := m.now()
	for _, feature := range features {
		timeout, ok := m.staleAfter[feature]
		if !ok {
			continue
		}
		if _, err := m.store.ExpireStaleUsage(ctx, userID, string(feature), now.Add(-timeout), now); err != nil {
			logger.GetDefaultLogger().WithFields(map[string]any{
				"user_id": userID,
				"feature": feature,
			}).ErrorWithErr(err, "Failed to expire stale usage reservations")
		}
	}
}

func (m *Meter) Limit(feature Feature) Limit {
	return m.limits[feature]
}

// Reservation is one admitted operation. It settles exactly once.
type Reservation struct {
	id        int64
	feature   Feature
	userID    int64
	startedAt time.Time
	collector *collector
	done      atomic.Bool
}

// Reserve charges units before the provider call. The returned context
// carries a collector for provider call metadata; pass it to the AI client.
func (m *Meter) Reserve(ctx context.Context, userID int64, feature Feature, units int64) (context.Context, *Reservation, error) {
	limit := m.limits[feature]
	if largest := admissibleMax(limit); largest > 0 && units > largest {
		return ctx, nil, &TooLargeError{Feature: feature, Max: largest}
	}

	m.expireStale(ctx, userID, feature)
	now := m.now()
	id, err := m.store.ReserveUsage(ctx, storage.UsageReserveParams{
		UserID: userID, Feature: string(feature), Day: DayStart(now), Units: units,
		UserLimit: limit.User, GlobalLimit: limit.Global, CreatedAt: now,
	})
	var limitErr *storage.UsageLimitError
	if errors.As(err, &limitErr) {
		quotaErr := &QuotaError{Feature: feature, Global: limitErr.Global, Limit: limit.User, ResetAt: ResetAt(now)}
		if !limitErr.Global {
			quotaErr.Used = limitErr.Used
		}
		return ctx, nil, quotaErr
	}
	if err != nil {
		return ctx, nil, fmt.Errorf("reserve %s usage: %w", feature, err)
	}

	c := &collector{}
	res := &Reservation{id: id, feature: feature, userID: userID, startedAt: now, collector: c}
	return context.WithValue(ctx, collectorKey{}, c), res, nil
}

// admissibleMax is the largest single request a user could ever be admitted
// for. It uses only the user's own and per-request limits so the public 413
// never reveals the global budget; a request larger than the global budget is
// rejected as an exhausted budget instead. A zero limit is reported as a quota
// rejection, not as too large.
func admissibleMax(limit Limit) int64 {
	if limit.User <= 0 {
		return 0
	}
	largest := limit.User
	if limit.PerRequest > 0 {
		largest = min(largest, limit.PerRequest)
	}
	return largest
}

// Age reports how long ago the reservation was admitted, on the meter's clock.
func (m *Meter) Age(res *Reservation) time.Duration {
	return m.now().Sub(res.startedAt)
}

// Release refunds a reservation whose provider call never started.
func (m *Meter) Release(ctx context.Context, res *Reservation) {
	m.settle(ctx, res, storage.UsageStatusReleased, storage.UsageCost{CostStatus: costStatusNone})
}

// Finish settles a reservation after the provider call. A nil callErr keeps
// the charge. Otherwise the user is refunded, and the global budget is
// refunded only when no attempt could have been billed.
func (m *Meter) Finish(ctx context.Context, res *Reservation, callErr error) {
	if res == nil {
		return
	}
	calls := res.collector.snapshot()
	status := storage.UsageStatusSettled
	if callErr != nil {
		status = failureStatus(calls, callErr)
	}
	cost := summarizeCalls(calls)
	cost.LatencyMS = m.now().Sub(res.startedAt).Milliseconds()
	m.settle(ctx, res, status, cost)
}

func (m *Meter) settle(ctx context.Context, res *Reservation, status string, cost storage.UsageCost) {
	if res == nil || !res.done.CompareAndSwap(false, true) {
		return
	}
	// Settlement must survive client disconnects and expired OCR deadlines.
	settleCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), settleTimeout)
	defer cancel()
	if _, err := m.store.SettleUsage(settleCtx, res.id, status, cost, m.now()); err != nil {
		logger.GetDefaultLogger().WithFields(map[string]any{
			"reservation_id": res.id,
			"feature":        res.feature,
			"status":         status,
		}).ErrorWithErr(err, "Failed to settle usage reservation; it stays charged")
	}
}

// cleanRejection reports error kinds that mean the provider did no billable
// work for the attempt.
func cleanRejection(kind ai.ErrorKind) bool {
	switch kind {
	case ai.ErrorKindConfig, ai.ErrorKindAuth, ai.ErrorKindRateLimit, ai.ErrorKindQuota:
		return true
	default:
		return false
	}
}

// failureStatus decides from structured attempt data, never message text.
// Observed attempts win. Clients that report no attempts (legacy minimax and
// gemini) may retry internally, so an earlier dispatched attempt could have
// been billed even when the final error is a clean rejection; only a
// configuration error raised before any HTTP status is treated as unbilled.
// Unknown and non-provider errors may have been billed.
func failureStatus(calls []ai.CallMetadata, callErr error) string {
	if len(calls) > 0 {
		for _, call := range calls {
			if !cleanRejection(call.ErrorKind) {
				return storage.UsageStatusFailed
			}
		}
		return storage.UsageStatusReleased
	}
	var providerErr *ai.ProviderError
	if errors.As(callErr, &providerErr) && providerErr.Kind == ai.ErrorKindConfig && providerErr.StatusCode == 0 {
		return storage.UsageStatusReleased
	}
	return storage.UsageStatusFailed
}

const (
	costStatusReported = "reported"
	costStatusMissing  = "missing"
	costStatusNone     = "none"
)

// summarizeCalls aggregates provider-reported usage. Cost is reported only
// when every attempt reported one in the same currency; nothing is estimated.
func summarizeCalls(calls []ai.CallMetadata) storage.UsageCost {
	cost := storage.UsageCost{CostStatus: costStatusMissing, Attempts: len(calls)}
	if len(calls) == 0 {
		return cost
	}
	last := calls[len(calls)-1]
	cost.Gateway, cost.Region, cost.Model = last.Gateway, last.Region, last.RequestedModel
	if last.ReportedModel != "" {
		cost.Model = last.ReportedModel
	}

	total := new(big.Rat)
	currency := calls[0].Currency
	reported := currency != ""
	for _, call := range calls {
		cost.InputTokens = addOptional(cost.InputTokens, call.InputTokens)
		cost.OutputTokens = addOptional(cost.OutputTokens, call.OutputTokens)
		cost.TotalTokens = addOptional(cost.TotalTokens, call.TotalTokens)
		cost.Characters = addOptional(cost.Characters, call.Characters)
		if call.Cost == nil || call.Currency != currency {
			reported = false
			continue
		}
		value, ok := new(big.Rat).SetString(call.Cost.String())
		if !ok || value.Sign() < 0 {
			reported = false
			continue
		}
		total.Add(total, value)
	}
	if reported {
		value := total.FloatString(8)
		cost.ProviderCost = &value
		cost.Currency = currency
		cost.CostStatus = costStatusReported
	}
	return cost
}

func addOptional(sum, value *int64) *int64 {
	if value == nil {
		return sum
	}
	next := *value
	if sum != nil {
		next += *sum
	}
	return &next
}

type collectorKey struct{}

type collector struct {
	mu    sync.Mutex
	calls []ai.CallMetadata
}

func (c *collector) snapshot() []ai.CallMetadata {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]ai.CallMetadata(nil), c.calls...)
}

// RecordCall attaches provider attempt metadata to the reservation whose
// context made the call. It is a no-op outside a reservation.
func RecordCall(ctx context.Context, meta ai.CallMetadata) {
	c, ok := ctx.Value(collectorKey{}).(*collector)
	if !ok {
		return
	}
	c.mu.Lock()
	c.calls = append(c.calls, meta)
	c.mu.Unlock()
}

// Calls returns the provider attempts recorded so far on a reservation's
// context, for reuse by other request-scoped consumers (e.g. provenance).
// It returns nil outside a reservation.
func Calls(ctx context.Context) []ai.CallMetadata {
	c, ok := ctx.Value(collectorKey{}).(*collector)
	if !ok {
		return nil
	}
	return c.snapshot()
}

// FeatureUsage is the caller's allowance for one feature today. Global totals
// are deliberately reduced to availability.
type FeatureUsage struct {
	Feature         Feature
	Unit            string
	Limit           int64
	Used            int64
	Remaining       int64
	GlobalAvailable bool
}

type Snapshot struct {
	ResetAt  time.Time
	Features []FeatureUsage
}

func (m *Meter) Snapshot(ctx context.Context, userID int64) (Snapshot, error) {
	m.expireStale(ctx, userID, Features...)
	now := m.now()
	counts, err := m.store.GetUsageCounts(ctx, userID, DayStart(now))
	if err != nil {
		return Snapshot{}, fmt.Errorf("read usage counts: %w", err)
	}
	snapshot := Snapshot{ResetAt: ResetAt(now), Features: make([]FeatureUsage, 0, len(Features))}
	for _, feature := range Features {
		limit := m.limits[feature]
		count := counts[string(feature)]
		snapshot.Features = append(snapshot.Features, FeatureUsage{
			Feature:         feature,
			Unit:            feature.Unit(),
			Limit:           limit.User,
			Used:            count.User,
			Remaining:       max(limit.User-count.User, 0),
			GlobalAvailable: count.Global < limit.Global,
		})
	}
	return snapshot, nil
}

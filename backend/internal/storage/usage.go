package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Reservation statuses. A reservation moves from reserved to exactly one
// final status; the final status decides which counters are refunded.
const (
	UsageStatusReserved = "reserved"
	// UsageStatusSettled keeps the charge on both counters.
	UsageStatusSettled = "settled"
	// UsageStatusFailed refunds the user but keeps the global charge, because
	// the provider may still bill dispatched work.
	UsageStatusFailed = "failed"
	// UsageStatusReleased refunds both counters: no billable work happened.
	UsageStatusReleased = "released"
)

// UsageReserveParams describes one admission against a user's allowance and
// the global budget for the same feature and UTC day.
type UsageReserveParams struct {
	UserID      int64
	Feature     string
	Day         time.Time
	Units       int64
	UserLimit   int64
	GlobalLimit int64
	CreatedAt   time.Time
}

// UsageLimitError reports a rejected admission and the usage it was checked
// against. Global is true when the shared budget, not the user, ran out.
type UsageLimitError struct {
	Global bool
	Used   int64
}

func (e *UsageLimitError) Error() string {
	if e.Global {
		return fmt.Sprintf("global usage budget exhausted (used %d)", e.Used)
	}
	return fmt.Sprintf("user usage quota exceeded (used %d)", e.Used)
}

// UsageCost is the provider usage recorded when a reservation settles.
// Nil values mean the provider did not report them.
type UsageCost struct {
	Gateway      string
	Region       string
	Model        string
	Attempts     int
	InputTokens  *int64
	OutputTokens *int64
	TotalTokens  *int64
	Characters   *int64
	// ProviderCost is a decimal string reported by the provider, never an estimate.
	ProviderCost *string
	Currency     string
	CostStatus   string
	LatencyMS    int64
}

// UsageCount is the units used on one day by a user and by everyone.
type UsageCount struct {
	User   int64
	Global int64
}

type UsageStore interface {
	// ReserveUsage atomically charges both counters or returns *UsageLimitError.
	ReserveUsage(ctx context.Context, p UsageReserveParams) (int64, error)
	// SettleUsage finalizes a reserved row once. It reports false when the
	// reservation was already settled or no longer exists.
	SettleUsage(ctx context.Context, reservationID int64, status string, cost UsageCost, settledAt time.Time) (bool, error)
	// GetUsageCounts returns per-feature counts for the user's day.
	GetUsageCounts(ctx context.Context, userID int64, day time.Time) (map[string]UsageCount, error)
	// ExpireStaleUsage settles the user's reservations of a feature that are
	// still reserved and were created before cutoff as failed: the user is
	// refunded on each reservation's own day and the global charge is kept.
	// It returns how many reservations it settled.
	ExpireStaleUsage(ctx context.Context, userID int64, feature string, cutoff, settledAt time.Time) (int64, error)
}

type postgresUsageStore struct {
	db *sql.DB
}

func NewPostgresUsageStore(db *sql.DB) UsageStore {
	return &postgresUsageStore{db: db}
}

func usageDay(day time.Time) string {
	return day.UTC().Format(time.DateOnly)
}

// ReserveUsage locks the user counter, then the global counter, in every
// transaction, so concurrent admissions cannot deadlock or overspend. The
// guard is a conditional UPDATE after a zero-row upsert so a fresh day still
// enforces the limit.
func (s *postgresUsageStore) ReserveUsage(ctx context.Context, p UsageReserveParams) (int64, error) {
	if p.Units <= 0 {
		return 0, fmt.Errorf("usage units must be positive")
	}
	day := usageDay(p.Day)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO usage_counters (user_id, feature, usage_day) VALUES ($1, $2, $3::date)
		ON CONFLICT DO NOTHING`, p.UserID, p.Feature, day); err != nil {
		return 0, fmt.Errorf("init user usage counter: %w", err)
	}
	if err := chargeCounter(ctx, tx, `
		UPDATE usage_counters SET used_units = used_units + $4
		WHERE user_id = $1 AND feature = $2 AND usage_day = $3::date AND used_units + $4 <= $5
		RETURNING used_units`, `
		SELECT used_units FROM usage_counters WHERE user_id = $1 AND feature = $2 AND usage_day = $3::date`,
		false, p.UserID, p.Feature, day, p.Units, p.UserLimit); err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO usage_global_counters (feature, usage_day) VALUES ($1, $2::date)
		ON CONFLICT DO NOTHING`, p.Feature, day); err != nil {
		return 0, fmt.Errorf("init global usage counter: %w", err)
	}
	if err := chargeCounter(ctx, tx, `
		UPDATE usage_global_counters SET used_units = used_units + $3
		WHERE feature = $1 AND usage_day = $2::date AND used_units + $3 <= $4
		RETURNING used_units`, `
		SELECT used_units FROM usage_global_counters WHERE feature = $1 AND usage_day = $2::date`,
		true, p.Feature, day, p.Units, p.GlobalLimit); err != nil {
		return 0, err
	}

	var id int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO usage_reservations (user_id, feature, usage_day, units, status, created_at)
		VALUES ($1, $2, $3::date, $4, 'reserved', $5)
		RETURNING id`, p.UserID, p.Feature, day, p.Units, p.CreatedAt).Scan(&id); err != nil {
		return 0, fmt.Errorf("insert usage reservation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// chargeCounter runs the conditional increment. The trailing two args of
// args are units and limit; the read-back query uses the leading key args.
func chargeCounter(ctx context.Context, tx *sql.Tx, update, readBack string, global bool, args ...any) error {
	var used int64
	err := tx.QueryRowContext(ctx, update, args...).Scan(&used)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("charge usage counter: %w", err)
	}
	keys := args[:len(args)-2]
	if err := tx.QueryRowContext(ctx, readBack, keys...).Scan(&used); err != nil {
		return fmt.Errorf("read usage counter: %w", err)
	}
	return &UsageLimitError{Global: global, Used: used}
}

func (s *postgresUsageStore) SettleUsage(ctx context.Context, reservationID int64, status string, cost UsageCost, settledAt time.Time) (bool, error) {
	refundUser, refundGlobal, err := usageRefunds(status)
	if err != nil {
		return false, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var (
		userID  int64
		feature string
		day     string
		units   int64
	)
	err = tx.QueryRowContext(ctx, `
		UPDATE usage_reservations SET
			status = $2, settled_at = $3, gateway = NULLIF($4, ''), region = NULLIF($5, ''),
			model = NULLIF($6, ''), attempts = $7, input_tokens = $8, output_tokens = $9,
			total_tokens = $10, characters = $11, provider_cost = $12::numeric,
			currency = NULLIF($13, ''), cost_status = $14, latency_ms = $15
		WHERE id = $1 AND status = 'reserved'
		RETURNING user_id, feature, to_char(usage_day, 'YYYY-MM-DD'), units`,
		reservationID, status, settledAt, cost.Gateway, cost.Region, cost.Model, cost.Attempts,
		cost.InputTokens, cost.OutputTokens, cost.TotalTokens, cost.Characters, cost.ProviderCost,
		cost.Currency, cost.CostStatus, cost.LatencyMS,
	).Scan(&userID, &feature, &day, &units)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("settle usage reservation: %w", err)
	}

	// Refund the reservation's own day so a late refund never credits a new day.
	if refundUser {
		if _, err := tx.ExecContext(ctx, `
			UPDATE usage_counters SET used_units = GREATEST(used_units - $4, 0)
			WHERE user_id = $1 AND feature = $2 AND usage_day = $3::date`, userID, feature, day, units); err != nil {
			return false, fmt.Errorf("refund user usage: %w", err)
		}
	}
	if refundGlobal {
		if _, err := tx.ExecContext(ctx, `
			UPDATE usage_global_counters SET used_units = GREATEST(used_units - $3, 0)
			WHERE feature = $1 AND usage_day = $2::date`, feature, day, units); err != nil {
			return false, fmt.Errorf("refund global usage: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func usageRefunds(status string) (refundUser, refundGlobal bool, err error) {
	switch status {
	case UsageStatusSettled:
		return false, false, nil
	case UsageStatusFailed:
		return true, false, nil
	case UsageStatusReleased:
		return true, true, nil
	default:
		return false, false, fmt.Errorf("invalid usage settlement status %q", status)
	}
}

func (s *postgresUsageStore) GetUsageCounts(ctx context.Context, userID int64, day time.Time) (map[string]UsageCount, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.feature, COALESCE(u.used_units, 0), g.used_units
		FROM usage_global_counters g
		LEFT JOIN usage_counters u
			ON u.feature = g.feature AND u.usage_day = g.usage_day AND u.user_id = $1
		WHERE g.usage_day = $2::date`, userID, usageDay(day))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]UsageCount)
	for rows.Next() {
		var feature string
		var count UsageCount
		if err := rows.Scan(&feature, &count.User, &count.Global); err != nil {
			return nil, err
		}
		counts[feature] = count
	}
	return counts, rows.Err()
}

// ExpireStaleUsage is one statement, so it is atomic. Stale rows are locked in
// id order and re-checked after any wait, so a concurrent Finish or expiry
// settles each reservation once. Like SettleUsage it locks the reservation
// before the counter, and UPDATE never recreates rows of a deleted user.
func (s *postgresUsageStore) ExpireStaleUsage(ctx context.Context, userID int64, feature string, cutoff, settledAt time.Time) (int64, error) {
	var expired int64
	err := s.db.QueryRowContext(ctx, `
		WITH stale AS (
			SELECT id FROM usage_reservations
			WHERE user_id = $1 AND feature = $2 AND status = 'reserved' AND created_at < $3
			ORDER BY id
			FOR UPDATE
		), expired AS (
			UPDATE usage_reservations r
			SET status = 'failed', settled_at = $4, cost_status = 'missing'
			FROM stale
			WHERE r.id = stale.id
			RETURNING r.usage_day, r.units
		), refunds AS (
			SELECT usage_day, SUM(units) AS units FROM expired GROUP BY usage_day
		), refunded AS (
			-- Data-modifying CTEs always run to completion, referenced or not.
			UPDATE usage_counters c
			SET used_units = GREATEST(c.used_units - refunds.units, 0)
			FROM refunds
			WHERE c.user_id = $1 AND c.feature = $2 AND c.usage_day = refunds.usage_day
		)
		SELECT COUNT(*) FROM expired`,
		userID, feature, cutoff, settledAt).Scan(&expired)
	if err != nil {
		return 0, fmt.Errorf("expire stale usage reservations: %w", err)
	}
	return expired, nil
}

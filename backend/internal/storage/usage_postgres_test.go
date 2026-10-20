package storage

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

var usageDay1 = time.Date(2026, 10, 20, 23, 59, 0, 0, time.UTC)

func reserveParams(userID int64, feature string, units, userLimit, globalLimit int64) UsageReserveParams {
	return UsageReserveParams{
		UserID: userID, Feature: feature, Day: usageDay1, Units: units,
		UserLimit: userLimit, GlobalLimit: globalLimit, CreatedAt: usageDay1,
	}
}

func requireLimitError(t *testing.T, err error, global bool, used int64) {
	t.Helper()
	var limitErr *UsageLimitError
	if !errors.As(err, &limitErr) {
		t.Fatalf("expected UsageLimitError, got %v", err)
	}
	if limitErr.Global != global || limitErr.Used != used {
		t.Fatalf("limit error = %+v, want global=%v used=%d", limitErr, global, used)
	}
}

func usageCounts(t *testing.T, store UsageStore, userID int64, day time.Time) map[string]UsageCount {
	t.Helper()
	counts, err := store.GetUsageCounts(context.Background(), userID, day)
	if err != nil {
		t.Fatalf("GetUsageCounts: %v", err)
	}
	return counts
}

func TestUsageReserveConcurrency_Postgres(t *testing.T) {
	db, pool := newMigratedPostgresStore(t)
	store := NewPostgresUsageStore(pool)
	user := createPGUser(t, db, "usage-concurrent")

	const attempts, limit = 25, 7
	var wg sync.WaitGroup
	var mu sync.Mutex
	admitted, rejected := 0, 0
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.ReserveUsage(context.Background(), reserveParams(user.ID, "explanation", 1, limit, 1000))
			mu.Lock()
			defer mu.Unlock()
			var limitErr *UsageLimitError
			switch {
			case err == nil:
				admitted++
			case errors.As(err, &limitErr):
				rejected++
			default:
				t.Errorf("unexpected reserve error: %v", err)
			}
		}()
	}
	wg.Wait()

	if admitted != limit || rejected != attempts-limit {
		t.Fatalf("admitted=%d rejected=%d, want %d/%d", admitted, rejected, limit, attempts-limit)
	}
	if got := usageCounts(t, store, user.ID, usageDay1)["explanation"]; got.User != limit || got.Global != limit {
		t.Fatalf("counts = %+v, want %d", got, limit)
	}
}

func TestUsageReserveGlobalBudgetConcurrency_Postgres(t *testing.T) {
	db, pool := newMigratedPostgresStore(t)
	store := NewPostgresUsageStore(pool)

	const users, globalLimit = 12, 5
	ids := make([]int64, users)
	for i := range ids {
		ids[i] = createPGUser(t, db, "usage-global-"+string(rune('a'+i))).ID
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	admitted := 0
	for _, id := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.ReserveUsage(context.Background(), reserveParams(id, "ocr", 1, 10, globalLimit))
			var limitErr *UsageLimitError
			if err != nil && (!errors.As(err, &limitErr) || !limitErr.Global) {
				t.Errorf("unexpected reserve error: %v", err)
			}
			if err == nil {
				mu.Lock()
				admitted++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if admitted != globalLimit {
		t.Fatalf("admitted=%d, want %d", admitted, globalLimit)
	}
	// Rejected users were never charged: the user charge rolled back with the global rejection.
	total := int64(0)
	for _, id := range ids {
		total += usageCounts(t, store, id, usageDay1)["ocr"].User
	}
	if total != globalLimit {
		t.Fatalf("sum of user counters = %d, want %d", total, globalLimit)
	}
}

func TestUsageReserveLimits_Postgres(t *testing.T) {
	db, pool := newMigratedPostgresStore(t)
	store := NewPostgresUsageStore(pool)
	ctx := context.Background()
	alice := createPGUser(t, db, "usage-alice")
	bob := createPGUser(t, db, "usage-bob")

	// A fresh day still enforces a zero allowance and an oversized request.
	_, err := store.ReserveUsage(ctx, reserveParams(alice.ID, "ocr", 1, 0, 100))
	requireLimitError(t, err, false, 0)
	_, err = store.ReserveUsage(ctx, reserveParams(alice.ID, "speech", 50, 40, 1000))
	requireLimitError(t, err, false, 0)
	_, err = store.ReserveUsage(ctx, reserveParams(alice.ID, "explanation", 1, 5, 0))
	requireLimitError(t, err, true, 0)

	// Per-user isolation with a shared global counter; features are independent.
	if _, err := store.ReserveUsage(ctx, reserveParams(alice.ID, "speech", 30, 40, 1000)); err != nil {
		t.Fatalf("alice speech: %v", err)
	}
	if _, err := store.ReserveUsage(ctx, reserveParams(bob.ID, "speech", 40, 40, 1000)); err != nil {
		t.Fatalf("bob speech: %v", err)
	}
	_, err = store.ReserveUsage(ctx, reserveParams(alice.ID, "speech", 11, 40, 1000))
	requireLimitError(t, err, false, 30)
	if _, err := store.ReserveUsage(ctx, reserveParams(alice.ID, "explanation", 1, 5, 100)); err != nil {
		t.Fatalf("speech usage must not block explanation: %v", err)
	}

	aliceCounts := usageCounts(t, store, alice.ID, usageDay1)
	if aliceCounts["speech"] != (UsageCount{User: 30, Global: 70}) {
		t.Fatalf("alice speech = %+v", aliceCounts["speech"])
	}
	if aliceCounts["explanation"] != (UsageCount{User: 1, Global: 1}) {
		t.Fatalf("alice explanation = %+v", aliceCounts["explanation"])
	}
	if _, ok := aliceCounts["ocr"]; ok {
		t.Fatalf("rejected OCR admission left a counter: %+v", aliceCounts["ocr"])
	}

	// A new UTC day starts from zero.
	if got := usageCounts(t, store, alice.ID, usageDay1.Add(time.Minute)); len(got) != 0 {
		t.Fatalf("next day counts = %+v, want empty", got)
	}
}

func TestUsageSettle_Postgres(t *testing.T) {
	db, pool := newMigratedPostgresStore(t)
	store := NewPostgresUsageStore(pool)
	ctx := context.Background()
	user := createPGUser(t, db, "usage-settle")
	reserve := func() int64 {
		t.Helper()
		id, err := store.ReserveUsage(ctx, reserveParams(user.ID, "ocr", 1, 10, 100))
		if err != nil {
			t.Fatalf("reserve: %v", err)
		}
		return id
	}
	settledID, failedID, releasedID := reserve(), reserve(), reserve()
	nextDay := usageDay1.Add(2 * time.Hour)
	cost := "0.00125000"
	tokens := int64(321)

	settle := func(id int64, status string, c UsageCost) bool {
		t.Helper()
		ok, err := store.SettleUsage(ctx, id, status, c, nextDay)
		if err != nil {
			t.Fatalf("settle %s: %v", status, err)
		}
		return ok
	}
	if !settle(settledID, UsageStatusSettled, UsageCost{Gateway: "openrouter", Model: "m", Attempts: 1, TotalTokens: &tokens, ProviderCost: &cost, Currency: "USD", CostStatus: "reported", LatencyMS: 900}) {
		t.Fatal("settled reservation was not updated")
	}
	if !settle(failedID, UsageStatusFailed, UsageCost{Attempts: 1, CostStatus: "missing"}) {
		t.Fatal("failed reservation was not updated")
	}
	if !settle(releasedID, UsageStatusReleased, UsageCost{CostStatus: "none"}) {
		t.Fatal("released reservation was not updated")
	}
	// Duplicate settlement is a no-op and cannot refund twice.
	if settle(releasedID, UsageStatusReleased, UsageCost{CostStatus: "none"}) {
		t.Fatal("second settlement was applied")
	}
	if _, err := store.SettleUsage(ctx, settledID, "bogus", UsageCost{}, nextDay); err == nil {
		t.Fatal("invalid status was accepted")
	}

	// Refunds hit the reservation's day, not the settlement day.
	if got := usageCounts(t, store, user.ID, usageDay1)["ocr"]; got != (UsageCount{User: 1, Global: 2}) {
		t.Fatalf("counts after settlement = %+v, want user=1 global=2", got)
	}

	var status, gateway, costStatus, providerCost string
	var totalTokens int64
	if err := pool.QueryRowContext(ctx, `
		SELECT status, gateway, cost_status, provider_cost::text, total_tokens
		FROM usage_reservations WHERE id = $1`, settledID).Scan(&status, &gateway, &costStatus, &providerCost, &totalTokens); err != nil {
		t.Fatalf("read settled reservation: %v", err)
	}
	if status != "settled" || gateway != "openrouter" || costStatus != "reported" || providerCost != "0.00125000" || totalTokens != 321 {
		t.Fatalf("settled row = %s %s %s %s %d", status, gateway, costStatus, providerCost, totalTokens)
	}
}

func TestUsagePersistenceAndAccountDeletion_Postgres(t *testing.T) {
	db, pool := newMigratedPostgresStore(t)
	ctx := context.Background()
	user := createPGUser(t, db, "usage-persist")
	inFlightID, err := NewPostgresUsageStore(pool).ReserveUsage(ctx, reserveParams(user.ID, "speech", 12, 100, 1000))
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}

	// A new store (as after a restart or on another replica) sees the same usage.
	restarted := NewPostgresUsageStore(pool)
	if got := usageCounts(t, restarted, user.ID, usageDay1)["speech"]; got != (UsageCount{User: 12, Global: 12}) {
		t.Fatalf("counts after restart = %+v", got)
	}
	_, err = restarted.ReserveUsage(ctx, reserveParams(user.ID, "speech", 89, 100, 1000))
	requireLimitError(t, err, false, 12)

	// The account is deleted while the provider call is still in flight.
	if err := db.DeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	// A late settlement, even a full release, must neither recreate private
	// rows nor refund global budget the in-flight call may have consumed.
	applied, err := restarted.SettleUsage(ctx, inFlightID, UsageStatusReleased, UsageCost{CostStatus: "none"}, usageDay1)
	if err != nil || applied {
		t.Fatalf("late settle after deletion = %v, %v; want no-op", applied, err)
	}
	var userRows, reservationRows, globalUsed int64
	if err := pool.QueryRowContext(ctx, `
		SELECT (SELECT COUNT(*) FROM usage_counters WHERE user_id = $1),
		       (SELECT COUNT(*) FROM usage_reservations WHERE user_id = $1),
		       (SELECT used_units FROM usage_global_counters WHERE feature = 'speech')`, user.ID,
	).Scan(&userRows, &reservationRows, &globalUsed); err != nil {
		t.Fatalf("read usage after deletion: %v", err)
	}
	if userRows != 0 || reservationRows != 0 || globalUsed != 12 {
		t.Fatalf("after deletion: user rows=%d reservations=%d global=%d, want 0/0/12", userRows, reservationRows, globalUsed)
	}
}

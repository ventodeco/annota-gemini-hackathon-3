package testutil

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gemini-hackathon/app/internal/storage"
)

// MemoryUsageReservation is a recorded reservation for assertions.
type MemoryUsageReservation struct {
	ID        int64
	UserID    int64
	Feature   string
	Day       string
	Units     int64
	Status    string
	Cost      storage.UsageCost
	CreatedAt time.Time
}

// MemoryUsageStore is an in-memory storage.UsageStore with the same
// admission and refund semantics as the PostgreSQL store.
type MemoryUsageStore struct {
	mu           sync.Mutex
	user         map[string]int64
	global       map[string]int64
	reservations map[int64]*MemoryUsageReservation
	nextID       int64
	// Err, when set, fails every call (accounting store unavailable).
	Err error
}

func NewMemoryUsageStore() *MemoryUsageStore {
	return &MemoryUsageStore{
		user:         make(map[string]int64),
		global:       make(map[string]int64),
		reservations: make(map[int64]*MemoryUsageReservation),
	}
}

func memoryUserKey(userID int64, feature, day string) string {
	return fmt.Sprintf("%d|%s|%s", userID, feature, day)
}

func memoryGlobalKey(feature, day string) string {
	return feature + "|" + day
}

func (s *MemoryUsageStore) ReserveUsage(_ context.Context, p storage.UsageReserveParams) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return 0, s.Err
	}
	day := p.Day.UTC().Format(time.DateOnly)
	userKey, globalKey := memoryUserKey(p.UserID, p.Feature, day), memoryGlobalKey(p.Feature, day)
	if s.user[userKey]+p.Units > p.UserLimit {
		return 0, &storage.UsageLimitError{Used: s.user[userKey]}
	}
	if s.global[globalKey]+p.Units > p.GlobalLimit {
		return 0, &storage.UsageLimitError{Global: true, Used: s.global[globalKey]}
	}
	s.user[userKey] += p.Units
	s.global[globalKey] += p.Units
	s.nextID++
	s.reservations[s.nextID] = &MemoryUsageReservation{
		ID: s.nextID, UserID: p.UserID, Feature: p.Feature, Day: day, Units: p.Units, Status: storage.UsageStatusReserved,
		CreatedAt: p.CreatedAt,
	}
	return s.nextID, nil
}

func (s *MemoryUsageStore) SettleUsage(_ context.Context, id int64, status string, cost storage.UsageCost, _ time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return false, s.Err
	}
	res, ok := s.reservations[id]
	if !ok || res.Status != storage.UsageStatusReserved {
		return false, nil
	}
	switch status {
	case storage.UsageStatusSettled:
	case storage.UsageStatusFailed:
		s.user[memoryUserKey(res.UserID, res.Feature, res.Day)] -= res.Units
	case storage.UsageStatusReleased:
		s.user[memoryUserKey(res.UserID, res.Feature, res.Day)] -= res.Units
		s.global[memoryGlobalKey(res.Feature, res.Day)] -= res.Units
	default:
		return false, fmt.Errorf("invalid usage settlement status %q", status)
	}
	res.Status = status
	res.Cost = cost
	return true, nil
}

func (s *MemoryUsageStore) GetUsageCounts(_ context.Context, userID int64, day time.Time) (map[string]storage.UsageCount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return nil, s.Err
	}
	date := day.UTC().Format(time.DateOnly)
	counts := make(map[string]storage.UsageCount)
	for key, used := range s.global {
		feature, keyDay, _ := strings.Cut(key, "|")
		if keyDay != date {
			continue
		}
		counts[feature] = storage.UsageCount{User: s.user[memoryUserKey(userID, feature, date)], Global: used}
	}
	return counts, nil
}

func (s *MemoryUsageStore) ExpireStaleUsage(_ context.Context, userID int64, feature string, cutoff, _ time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return 0, s.Err
	}
	var expired int64
	for _, res := range s.reservations {
		if res.UserID != userID || res.Feature != feature || res.Status != storage.UsageStatusReserved || !res.CreatedAt.Before(cutoff) {
			continue
		}
		res.Status = storage.UsageStatusFailed
		res.Cost = storage.UsageCost{CostStatus: "missing"}
		s.user[memoryUserKey(res.UserID, res.Feature, res.Day)] -= res.Units
		expired++
	}
	return expired, nil
}

// Reservations returns a copy of every reservation in creation order.
func (s *MemoryUsageStore) Reservations() []MemoryUsageReservation {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]MemoryUsageReservation, 0, len(s.reservations))
	for id := int64(1); id <= s.nextID; id++ {
		if res, ok := s.reservations[id]; ok {
			out = append(out, *res)
		}
	}
	return out
}

package auth_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gemini-hackathon/app/internal/auth"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/storage"
	"github.com/gemini-hackathon/app/internal/testutil"
)

type fakeDirectory struct {
	mu         sync.Mutex
	identities map[string]*auth.ClerkIdentity
	err        error
	gets       int
}

func (f *fakeDirectory) GetIdentity(_ context.Context, id string) (*auth.ClerkIdentity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gets++
	if f.err != nil {
		return nil, f.err
	}
	identity, ok := f.identities[id]
	if !ok {
		return nil, auth.ErrIdentityNotFound
	}
	return identity, nil
}

func (f *fakeDirectory) DeleteUser(context.Context, string) error { return nil }

func TestIdentityResolverResolve(t *testing.T) {
	ctx := context.Background()

	seedLegacy := func(t *testing.T, db *testutil.MockDB, email, googleSub string) *models.User {
		t.Helper()
		user := &models.User{Email: email, Provider: "google", ProviderID: googleSub, PreferredLanguage: "JP",
			CreatedAt: time.Now(), UpdatedAt: time.Now()}
		if err := db.CreateUser(ctx, user); err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		return user
	}

	t.Run("new email user is provisioned once and reused", func(t *testing.T) {
		db := testutil.NewMockDB()
		dir := &fakeDirectory{identities: map[string]*auth.ClerkIdentity{
			"user_new": {UserID: "user_new", VerifiedPrimaryEmail: "new@example.com"},
		}}
		resolver := auth.NewIdentityResolver(db, dir)

		first, err := resolver.Resolve(ctx, "user_new")
		if err != nil || first.UserID == 0 || first.Deleted {
			t.Fatalf("first resolve: %+v err=%v", first, err)
		}
		second, err := resolver.Resolve(ctx, "user_new")
		if err != nil || second.UserID != first.UserID {
			t.Fatalf("second resolve: %+v err=%v", second, err)
		}
		if dir.gets != 1 {
			t.Fatalf("Clerk Backend API should be called only on first provisioning, got %d", dir.gets)
		}
	})

	t.Run("legacy google user keeps numeric id", func(t *testing.T) {
		db := testutil.NewMockDB()
		legacy := seedLegacy(t, db, "legacy@gmail.com", "google-sub-1")
		dir := &fakeDirectory{identities: map[string]*auth.ClerkIdentity{
			"user_legacy": {UserID: "user_legacy", VerifiedPrimaryEmail: "legacy@gmail.com", GoogleSubject: "google-sub-1"},
		}}
		got, err := auth.NewIdentityResolver(db, dir).Resolve(ctx, "user_legacy")
		if err != nil || got.UserID != legacy.ID {
			t.Fatalf("expected legacy id %d, got %+v err=%v", legacy.ID, got, err)
		}
	})

	t.Run("email-only sign-in never takes over a legacy google profile", func(t *testing.T) {
		db := testutil.NewMockDB()
		seedLegacy(t, db, "Legacy@Gmail.com", "google-sub-1")
		dir := &fakeDirectory{identities: map[string]*auth.ClerkIdentity{
			"user_email": {UserID: "user_email", VerifiedPrimaryEmail: "legacy@gmail.com"},
		}}
		_, err := auth.NewIdentityResolver(db, dir).Resolve(ctx, "user_email")
		if !errors.Is(err, storage.ErrLegacyLinkRequired) {
			t.Fatalf("expected ErrLegacyLinkRequired, got %v", err)
		}
	})

	t.Run("unverified email is refused", func(t *testing.T) {
		db := testutil.NewMockDB()
		dir := &fakeDirectory{identities: map[string]*auth.ClerkIdentity{"user_x": {UserID: "user_x"}}}
		_, err := auth.NewIdentityResolver(db, dir).Resolve(ctx, "user_x")
		if !errors.Is(err, auth.ErrVerifiedEmailRequired) {
			t.Fatalf("expected ErrVerifiedEmailRequired, got %v", err)
		}
	})

	t.Run("clerk outage only affects first-time users", func(t *testing.T) {
		db := testutil.NewMockDB()
		dir := &fakeDirectory{identities: map[string]*auth.ClerkIdentity{
			"user_known": {UserID: "user_known", VerifiedPrimaryEmail: "known@example.com"},
		}}
		resolver := auth.NewIdentityResolver(db, dir)
		known, err := resolver.Resolve(ctx, "user_known")
		if err != nil {
			t.Fatalf("seed resolve: %v", err)
		}
		dir.err = auth.ErrIdentityUnavailable
		again, err := resolver.Resolve(ctx, "user_known")
		if err != nil || again.UserID != known.UserID {
			t.Fatalf("existing user should resolve during outage: %+v err=%v", again, err)
		}
		if _, err := resolver.Resolve(ctx, "user_brand_new"); !errors.Is(err, auth.ErrIdentityUnavailable) {
			t.Fatalf("expected ErrIdentityUnavailable for first-time user, got %v", err)
		}
	})

	t.Run("deleted subject is never recreated", func(t *testing.T) {
		db := testutil.NewMockDB()
		dir := &fakeDirectory{identities: map[string]*auth.ClerkIdentity{
			"user_gone": {UserID: "user_gone", VerifiedPrimaryEmail: "gone@example.com"},
		}}
		resolver := auth.NewIdentityResolver(db, dir)
		first, err := resolver.Resolve(ctx, "user_gone")
		if err != nil {
			t.Fatalf("seed resolve: %v", err)
		}
		if err := db.DeleteUserAndTombstoneClerk(ctx, first.UserID, "user_gone"); err != nil {
			t.Fatalf("delete: %v", err)
		}
		got, err := resolver.Resolve(ctx, "user_gone")
		if err != nil || !got.Deleted || got.UserID != 0 {
			t.Fatalf("expected deleted resolution, got %+v err=%v", got, err)
		}
	})
}

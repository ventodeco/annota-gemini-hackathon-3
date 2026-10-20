package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwks"
	"github.com/go-jose/go-jose/v3"
	josejwt "github.com/go-jose/go-jose/v3/jwt"
)

const (
	testIssuer = "https://annota-test.clerk.accounts.dev"
	testOrigin = "http://localhost:5173"
	testKID    = "ins_test_key"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type fakeJWKS struct {
	mu    sync.Mutex
	set   *clerk.JSONWebKeySet
	err   error
	calls int
	// block, when set, holds Get until it is closed.
	block chan struct{}
	// started receives a value when Get begins (if non-nil).
	started chan struct{}
}

func (f *fakeJWKS) Get(ctx context.Context, _ *jwks.GetParams) (*clerk.JSONWebKeySet, error) {
	f.mu.Lock()
	f.calls++
	block, started := f.block, f.started
	f.mu.Unlock()
	if started != nil {
		started <- struct{}{}
	}
	if block != nil {
		<-block
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.set, f.err
}

func (f *fakeJWKS) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func newTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key
}

func publicJWK(key *rsa.PrivateKey) *clerk.JSONWebKey {
	return &clerk.JSONWebKey{Key: &key.PublicKey, KeyID: testKID, Algorithm: "RS256"}
}

type tokenClaims struct {
	josejwt.Claims
	AuthorizedParty string `json:"azp,omitempty"`
	SessionID       string `json:"sid,omitempty"`
}

func signToken(t *testing.T, alg jose.SignatureAlgorithm, key any, kid string, claims tokenClaims) string {
	t.Helper()
	opts := (&jose.SignerOptions{}).WithType("JWT")
	if kid != "" {
		opts = opts.WithHeader(jose.HeaderKey("kid"), kid)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: alg, Key: key}, opts)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	token, err := josejwt.Signed(signer).Claims(claims).CompactSerialize()
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func validClaims(now time.Time) tokenClaims {
	return tokenClaims{
		Claims: josejwt.Claims{
			Issuer:    testIssuer,
			Subject:   "user_abc",
			IssuedAt:  josejwt.NewNumericDate(now.Add(-10 * time.Second)),
			NotBefore: josejwt.NewNumericDate(now.Add(-10 * time.Second)),
			Expiry:    josejwt.NewNumericDate(now.Add(50 * time.Second)),
		},
		AuthorizedParty: testOrigin,
		SessionID:       "sess_1",
	}
}

func TestSessionVerifierVerify(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	key := newTestKey(t)
	otherKey := newTestKey(t)

	verifier, err := NewSessionVerifier(SessionVerifierConfig{
		Issuer:            testIssuer,
		AuthorizedParties: []string{testOrigin},
		Keys:              &staticKeySource{key: publicJWK(key)},
		Clock:             fixedClock{now: now},
	})
	if err != nil {
		t.Fatalf("NewSessionVerifier: %v", err)
	}

	tests := []struct {
		name  string
		token func() string
		want  string
	}{
		{name: "valid", want: "user_abc", token: func() string {
			return signToken(t, jose.RS256, key, testKID, validClaims(now))
		}},
		{name: "garbled", token: func() string { return "not-a-jwt" }},
		{name: "expired", token: func() string {
			c := validClaims(now)
			c.Expiry = josejwt.NewNumericDate(now.Add(-time.Minute))
			return signToken(t, jose.RS256, key, testKID, c)
		}},
		{name: "missing expiry", token: func() string {
			c := validClaims(now)
			c.Expiry = nil
			return signToken(t, jose.RS256, key, testKID, c)
		}},
		{name: "not yet valid", token: func() string {
			c := validClaims(now)
			c.NotBefore = josejwt.NewNumericDate(now.Add(time.Minute))
			return signToken(t, jose.RS256, key, testKID, c)
		}},
		{name: "wrong signing key", token: func() string {
			return signToken(t, jose.RS256, otherKey, testKID, validClaims(now))
		}},
		{name: "legacy HS256 token", token: func() string {
			return signToken(t, jose.HS256, []byte("01234567890123456789012345678912"), testKID, validClaims(now))
		}},
		{name: "other clerk instance issuer", token: func() string {
			c := validClaims(now)
			c.Issuer = "https://someone-else.clerk.accounts.dev"
			return signToken(t, jose.RS256, key, testKID, c)
		}},
		{name: "foreign authorized party", token: func() string {
			c := validClaims(now)
			c.AuthorizedParty = "https://evil.example"
			return signToken(t, jose.RS256, key, testKID, c)
		}},
		{name: "missing authorized party", token: func() string {
			c := validClaims(now)
			c.AuthorizedParty = ""
			return signToken(t, jose.RS256, key, testKID, c)
		}},
		{name: "missing subject", token: func() string {
			c := validClaims(now)
			c.Subject = ""
			return signToken(t, jose.RS256, key, testKID, c)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := verifier.Verify(context.Background(), tt.token())
			if tt.want == "" {
				if !errors.Is(err, ErrInvalidSession) {
					t.Fatalf("expected ErrInvalidSession, got subject=%q err=%v", got, err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("expected subject %q, got %q err=%v", tt.want, got, err)
			}
		})
	}
}

func TestNewSessionVerifierRejectsUnsafeConfig(t *testing.T) {
	keys := &staticKeySource{}
	cases := []SessionVerifierConfig{
		{Issuer: "http://insecure.clerk.accounts.dev", AuthorizedParties: []string{testOrigin}, Keys: keys},
		{Issuer: testIssuer, Keys: keys},
		{Issuer: testIssuer, AuthorizedParties: []string{"*"}, Keys: keys},
		{Issuer: testIssuer, AuthorizedParties: []string{testOrigin}},
	}
	for i, cfg := range cases {
		if _, err := NewSessionVerifier(cfg); err == nil {
			t.Fatalf("case %d: expected error", i)
		}
	}
}

func TestJWKSKeySourceCachesAndBoundsRefetch(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	key := newTestKey(t)
	fetcher := &fakeJWKS{set: &clerk.JSONWebKeySet{Keys: []*clerk.JSONWebKey{publicJWK(key)}}}
	source := newJWKSKeySource(fetcher, time.Hour, time.Minute, func() time.Time { return now })
	ctx := context.Background()

	if _, err := source.Key(ctx, testKID); err != nil {
		t.Fatalf("first lookup: %v", err)
	}
	if _, err := source.Key(ctx, testKID); err != nil {
		t.Fatalf("cached lookup: %v", err)
	}
	if fetcher.callCount() != 1 {
		t.Fatalf("expected one JWKS fetch, got %d", fetcher.callCount())
	}

	// Unknown kids inside the refresh window are rejected without refetching.
	if _, err := source.Key(ctx, "forged"); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("expected ErrInvalidSession for unknown kid, got %v", err)
	}
	if fetcher.callCount() != 1 {
		t.Fatalf("unknown kid refetched inside refresh window: %d calls", fetcher.callCount())
	}

	// After the TTL a fetch failure keeps serving the trusted cached key.
	now = now.Add(2 * time.Hour)
	fetcher.err = errors.New("clerk down")
	if _, err := source.Key(ctx, testKID); err != nil {
		t.Fatalf("expected stale key during outage, got %v", err)
	}
	// An unknown kid during an outage cannot be checked either way.
	if _, err := source.Key(ctx, "rotated"); !errors.Is(err, ErrVerifierUnavailable) {
		t.Fatalf("expected ErrVerifierUnavailable, got %v", err)
	}
	// Failed attempts are throttled too: repeated requests during the outage
	// do not each call Clerk.
	callsAfterFailure := fetcher.callCount()
	for range 5 {
		if _, err := source.Key(ctx, testKID); err != nil {
			t.Fatalf("expected stale key during throttled outage, got %v", err)
		}
	}
	if got := fetcher.callCount(); got != callsAfterFailure {
		t.Fatalf("failed refresh was retried inside backoff window: %d -> %d calls", callsAfterFailure, got)
	}
	// After the backoff window a recovered Clerk refreshes the keys again.
	now = now.Add(2 * time.Minute)
	fetcher.mu.Lock()
	fetcher.err = nil
	fetcher.mu.Unlock()
	if _, err := source.Key(ctx, testKID); err != nil || fetcher.callCount() != callsAfterFailure+1 {
		t.Fatalf("expected one recovery fetch, got err=%v calls=%d", err, fetcher.callCount())
	}
}

func TestJWKSKeySourceDoesNotBlockTrustedKeysOnRefresh(t *testing.T) {
	var mu sync.Mutex
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	key := newTestKey(t)
	fetcher := &fakeJWKS{set: &clerk.JSONWebKeySet{Keys: []*clerk.JSONWebKey{publicJWK(key)}}}
	source := newJWKSKeySource(fetcher, time.Hour, time.Minute, clock)
	ctx := context.Background()
	if _, err := source.Key(ctx, testKID); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Expire the cache and make the next refresh hang.
	mu.Lock()
	now = now.Add(2 * time.Hour)
	mu.Unlock()
	fetcher.mu.Lock()
	fetcher.block = make(chan struct{})
	fetcher.started = make(chan struct{}, 1)
	release, started := fetcher.block, fetcher.started
	fetcher.mu.Unlock()

	refreshDone := make(chan error, 1)
	go func() {
		_, err := source.Key(ctx, testKID)
		refreshDone <- err
	}()
	<-started

	// While the refresh is in flight, trusted keys return immediately and no
	// second fetch starts; unknown keys give up when their request ends.
	for range 3 {
		if _, err := source.Key(ctx, testKID); err != nil {
			t.Fatalf("trusted key blocked or failed during refresh: %v", err)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := source.Key(cancelled, "unknown"); !errors.Is(err, ErrVerifierUnavailable) {
		t.Fatalf("expected ErrVerifierUnavailable for cancelled wait, got %v", err)
	}
	if got := fetcher.callCount(); got != 2 {
		t.Fatalf("expected exactly one in-flight refresh, got %d fetches", got)
	}

	close(release)
	if err := <-refreshDone; err != nil {
		t.Fatalf("refreshing caller: %v", err)
	}
}

func TestIssuerFromPublishableKey(t *testing.T) {
	encode := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	tests := []struct {
		key     string
		want    string
		wantErr bool
	}{
		{key: "pk_test_" + encode("annota-test.clerk.accounts.dev$"), want: testIssuer},
		{key: "pk_live_" + base64.RawStdEncoding.EncodeToString([]byte("clerk.annota.app$")), want: "https://clerk.annota.app"},
		{key: "", wantErr: true},
		{key: "sk_test_" + encode("annota-test.clerk.accounts.dev$"), wantErr: true},
		{key: "pk_test_" + encode("annota-test.clerk.accounts.dev"), wantErr: true},
		{key: "pk_test_" + encode("evil.example/path$"), wantErr: true},
		{key: "pk_test_%%%", wantErr: true},
	}
	for _, tt := range tests {
		got, err := IssuerFromPublishableKey(tt.key)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("%q: expected error, got %q", tt.key, got)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Fatalf("%q: got %q err=%v, want %q", tt.key, got, err, tt.want)
		}
	}
}

func TestIdentityFromClerkUserUsesOnlyVerifiedFacts(t *testing.T) {
	primaryID := "idn_primary"
	image := "https://img.clerk.com/avatar"
	verified := &clerk.Verification{Status: "verified"}
	unverified := &clerk.Verification{Status: "unverified"}

	identity := identityFromClerkUser(&clerk.User{
		ID:                    "user_abc",
		ImageURL:              &image,
		PrimaryEmailAddressID: &primaryID,
		EmailAddresses: []*clerk.EmailAddress{
			{ID: "idn_other", EmailAddress: "other@example.com", Verification: verified},
			{ID: primaryID, EmailAddress: "learner@example.com", Verification: verified},
		},
		ExternalAccounts: []*clerk.ExternalAccount{
			{Provider: "oauth_github", ProviderUserID: "gh-1", Verification: verified},
			{Provider: "oauth_google", ProviderUserID: "google-sub-1", Verification: verified},
		},
	})
	if identity.VerifiedPrimaryEmail != "learner@example.com" || identity.GoogleSubject != "google-sub-1" ||
		identity.AvatarURL != image {
		t.Fatalf("unexpected identity %+v", identity)
	}

	unverifiedIdentity := identityFromClerkUser(&clerk.User{
		ID:                    "user_def",
		PrimaryEmailAddressID: &primaryID,
		EmailAddresses:        []*clerk.EmailAddress{{ID: primaryID, EmailAddress: "x@example.com", Verification: unverified}},
		ExternalAccounts:      []*clerk.ExternalAccount{{Provider: "oauth_google", ProviderUserID: "google-sub-2", Verification: unverified}},
	})
	if unverifiedIdentity.VerifiedPrimaryEmail != "" || unverifiedIdentity.GoogleSubject != "" {
		t.Fatalf("unverified facts must be ignored: %+v", unverifiedIdentity)
	}
}

type fakeUserAPI struct {
	getErr    error
	deleteErr error
}

func (f fakeUserAPI) Get(context.Context, string) (*clerk.User, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return &clerk.User{ID: "user_abc"}, nil
}

func (f fakeUserAPI) Delete(context.Context, string) (*clerk.DeletedResource, error) {
	return nil, f.deleteErr
}

func TestClerkDirectoryMapsErrors(t *testing.T) {
	ctx := context.Background()
	notFound := &clerk.APIErrorResponse{HTTPStatusCode: 404}

	if _, err := (&clerkDirectory{users: fakeUserAPI{getErr: notFound}}).GetIdentity(ctx, "user_abc"); !errors.Is(err, ErrIdentityNotFound) {
		t.Fatalf("expected ErrIdentityNotFound, got %v", err)
	}
	if _, err := (&clerkDirectory{users: fakeUserAPI{getErr: errors.New("timeout")}}).GetIdentity(ctx, "user_abc"); !errors.Is(err, ErrIdentityUnavailable) {
		t.Fatalf("expected ErrIdentityUnavailable, got %v", err)
	}
	if err := (&clerkDirectory{users: fakeUserAPI{deleteErr: notFound}}).DeleteUser(ctx, "user_abc"); err != nil {
		t.Fatalf("deleting an already-deleted Clerk user should succeed, got %v", err)
	}
	if err := (&clerkDirectory{users: fakeUserAPI{deleteErr: errors.New("500")}}).DeleteUser(ctx, "user_abc"); !errors.Is(err, ErrIdentityUnavailable) {
		t.Fatalf("expected ErrIdentityUnavailable, got %v", err)
	}
}

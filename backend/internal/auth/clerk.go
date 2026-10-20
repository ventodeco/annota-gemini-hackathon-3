package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwks"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/clerk/clerk-sdk-go/v2/user"
)

var (
	// ErrInvalidSession means the bearer token is not a current session
	// token issued by this application's Clerk instance.
	ErrInvalidSession = errors.New("invalid session token")
	// ErrVerifierUnavailable means signing keys could not be loaded, so
	// the token could not be checked either way.
	ErrVerifierUnavailable = errors.New("session verification unavailable")
	// ErrIdentityNotFound means Clerk has no user for the verified subject.
	ErrIdentityNotFound = errors.New("clerk identity not found")
	// ErrIdentityUnavailable means the Clerk Backend API could not be reached.
	ErrIdentityUnavailable = errors.New("clerk identity service unavailable")
)

const googleOAuthProvider = "oauth_google"

// IssuerFromPublishableKey derives the Frontend API issuer URL encoded in a
// Clerk publishable key (pk_test_/pk_live_ + base64("<host>$")).
func IssuerFromPublishableKey(publishableKey string) (string, error) {
	encoded, ok := strings.CutPrefix(publishableKey, "pk_test_")
	if !ok {
		encoded, ok = strings.CutPrefix(publishableKey, "pk_live_")
	}
	if !ok || encoded == "" {
		return "", fmt.Errorf("publishable key must start with pk_test_ or pk_live_")
	}
	decoded, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(encoded, "="))
	if err != nil {
		return "", fmt.Errorf("publishable key is not valid base64")
	}
	host, ok := strings.CutSuffix(string(decoded), "$")
	if !ok || host == "" || strings.ContainsAny(host, "/$ ") {
		return "", fmt.Errorf("publishable key does not encode a Clerk frontend host")
	}
	return "https://" + host, nil
}

// KeySource returns the public key that signed a session token.
type KeySource interface {
	Key(ctx context.Context, kid string) (*clerk.JSONWebKey, error)
}

type staticKeySource struct {
	key *clerk.JSONWebKey
}

// NewStaticKeySource uses a PEM public key (CLERK_JWT_KEY) for networkless
// verification.
func NewStaticKeySource(pemKey string) (KeySource, error) {
	if !strings.HasPrefix(pemKey, "-----BEGIN") {
		pemKey = "-----BEGIN PUBLIC KEY-----\n" + pemKey + "\n-----END PUBLIC KEY-----"
	}
	key, err := clerk.JSONWebKeyFromPEM(pemKey)
	if err != nil {
		return nil, fmt.Errorf("parse CLERK_JWT_KEY: %w", err)
	}
	return &staticKeySource{key: key}, nil
}

func (s *staticKeySource) Key(context.Context, string) (*clerk.JSONWebKey, error) {
	return s.key, nil
}

type jwksFetcher interface {
	Get(ctx context.Context, params *jwks.GetParams) (*clerk.JSONWebKeySet, error)
}

// jwksKeySource caches the instance JWKS. Unknown key IDs trigger a refetch at
// most once per minRefresh so forged kids cannot hammer the Clerk API.
type jwksKeySource struct {
	client     jwksFetcher
	ttl        time.Duration
	minRefresh time.Duration
	now        func() time.Time

	mu        sync.Mutex
	keys      map[string]*clerk.JSONWebKey
	fetchedAt time.Time
}

// NewJWKSKeySource fetches signing keys from the Clerk Backend API.
func NewJWKSKeySource(client *jwks.Client) KeySource {
	return newJWKSKeySource(client, time.Hour, time.Minute, time.Now)
}

func newJWKSKeySource(client jwksFetcher, ttl, minRefresh time.Duration, now func() time.Time) *jwksKeySource {
	return &jwksKeySource{client: client, ttl: ttl, minRefresh: minRefresh, now: now}
}

func (s *jwksKeySource) Key(ctx context.Context, kid string) (*clerk.JSONWebKey, error) {
	if kid == "" {
		return nil, ErrInvalidSession
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	key, known := s.keys[kid]
	fresh := !s.fetchedAt.IsZero() && now.Sub(s.fetchedAt) < s.ttl
	if known && fresh {
		return key, nil
	}
	if !known && fresh && now.Sub(s.fetchedAt) < s.minRefresh {
		return nil, ErrInvalidSession
	}

	set, err := s.client.Get(ctx, &jwks.GetParams{})
	if err != nil || set == nil {
		if known {
			// Keep serving a previously trusted key through a transient outage.
			return key, nil
		}
		return nil, ErrVerifierUnavailable
	}
	keys := make(map[string]*clerk.JSONWebKey, len(set.Keys))
	for _, k := range set.Keys {
		if k != nil && k.KeyID != "" {
			keys[k.KeyID] = k
		}
	}
	s.keys = keys
	s.fetchedAt = now
	if key, ok := keys[kid]; ok {
		return key, nil
	}
	return nil, ErrInvalidSession
}

type SessionVerifierConfig struct {
	// Issuer is the exact Frontend API URL of this application's instance.
	Issuer string
	// AuthorizedParties are the frontend origins allowed in the azp claim.
	AuthorizedParties []string
	Keys              KeySource
	Clock             clerk.Clock
	Leeway            time.Duration
}

// SessionVerifier validates Clerk session JWTs for this application only.
type SessionVerifier struct {
	issuer  string
	parties map[string]struct{}
	keys    KeySource
	clock   clerk.Clock
	leeway  time.Duration
}

func NewSessionVerifier(cfg SessionVerifierConfig) (*SessionVerifier, error) {
	if !strings.HasPrefix(cfg.Issuer, "https://") {
		return nil, fmt.Errorf("clerk issuer must be an https URL")
	}
	if len(cfg.AuthorizedParties) == 0 {
		return nil, fmt.Errorf("at least one authorized party is required")
	}
	if cfg.Keys == nil {
		return nil, fmt.Errorf("a key source is required")
	}
	parties := make(map[string]struct{}, len(cfg.AuthorizedParties))
	for _, p := range cfg.AuthorizedParties {
		if p == "" || p == "*" {
			return nil, fmt.Errorf("authorized parties must be explicit origins")
		}
		parties[p] = struct{}{}
	}
	clock := cfg.Clock
	if clock == nil {
		clock = clerk.NewClock()
	}
	return &SessionVerifier{
		issuer:  cfg.Issuer,
		parties: parties,
		keys:    cfg.Keys,
		clock:   clock,
		leeway:  cfg.Leeway,
	}, nil
}

// Verify returns the Clerk user ID (sub) of a valid session token.
func (v *SessionVerifier) Verify(ctx context.Context, token string) (string, error) {
	decoded, err := jwt.Decode(ctx, &jwt.DecodeParams{Token: token})
	if err != nil {
		return "", ErrInvalidSession
	}
	key, err := v.keys.Key(ctx, decoded.KeyID)
	if err != nil {
		if errors.Is(err, ErrVerifierUnavailable) {
			return "", err
		}
		return "", ErrInvalidSession
	}
	issuer := v.issuer
	claims, err := jwt.Verify(ctx, &jwt.VerifyParams{
		Token:  token,
		JWK:    key,
		Clock:  v.clock,
		Leeway: v.leeway,
		// The SDK compares iss for equality with ProxyURL; without it, any
		// Clerk instance's issuer would be accepted.
		ProxyURL: &issuer,
		AuthorizedPartyHandler: func(azp string) bool {
			_, ok := v.parties[azp]
			return ok
		},
	})
	if err != nil {
		return "", ErrInvalidSession
	}
	if claims.Subject == "" || claims.Expiry == nil {
		return "", ErrInvalidSession
	}
	return claims.Subject, nil
}

// ClerkIdentity holds the verified identity facts ANNOTA relies on.
type ClerkIdentity struct {
	UserID               string
	VerifiedPrimaryEmail string
	AvatarURL            string
	// GoogleSubject is the Google account ID from a verified Google
	// connection, used to link legacy Google-only ANNOTA profiles.
	GoogleSubject string
}

// IdentityDirectory reads and deletes users through the Clerk Backend API.
type IdentityDirectory interface {
	GetIdentity(ctx context.Context, clerkUserID string) (*ClerkIdentity, error)
	DeleteUser(ctx context.Context, clerkUserID string) error
}

type clerkUserAPI interface {
	Get(ctx context.Context, id string) (*clerk.User, error)
	Delete(ctx context.Context, id string) (*clerk.DeletedResource, error)
}

type clerkDirectory struct {
	users clerkUserAPI
}

func NewClerkDirectory(client *user.Client) IdentityDirectory {
	return &clerkDirectory{users: client}
}

func (d *clerkDirectory) GetIdentity(ctx context.Context, clerkUserID string) (*ClerkIdentity, error) {
	u, err := d.users.Get(ctx, clerkUserID)
	if err != nil {
		if isClerkNotFound(err) {
			return nil, ErrIdentityNotFound
		}
		return nil, fmt.Errorf("%w: %v", ErrIdentityUnavailable, err)
	}
	return identityFromClerkUser(u), nil
}

func (d *clerkDirectory) DeleteUser(ctx context.Context, clerkUserID string) error {
	if _, err := d.users.Delete(ctx, clerkUserID); err != nil {
		if isClerkNotFound(err) {
			return nil
		}
		return fmt.Errorf("%w: %v", ErrIdentityUnavailable, err)
	}
	return nil
}

func isClerkNotFound(err error) bool {
	var apiErr *clerk.APIErrorResponse
	return errors.As(err, &apiErr) && apiErr.HTTPStatusCode == http.StatusNotFound
}

func identityFromClerkUser(u *clerk.User) *ClerkIdentity {
	identity := &ClerkIdentity{UserID: u.ID}
	if u.ImageURL != nil {
		identity.AvatarURL = *u.ImageURL
	}
	if u.PrimaryEmailAddressID != nil {
		for _, email := range u.EmailAddresses {
			if email != nil && email.ID == *u.PrimaryEmailAddressID && isVerified(email.Verification) {
				identity.VerifiedPrimaryEmail = email.EmailAddress
			}
		}
	}
	for _, account := range u.ExternalAccounts {
		if account != nil && account.Provider == googleOAuthProvider &&
			account.ProviderUserID != "" && isVerified(account.Verification) {
			identity.GoogleSubject = account.ProviderUserID
			break
		}
	}
	return identity
}

func isVerified(v *clerk.Verification) bool {
	return v != nil && v.Status == "verified"
}

package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gemini-hackathon/app/internal/auth"
	"github.com/gemini-hackathon/app/internal/httputil"
	"github.com/gemini-hackathon/app/internal/logger"
	"github.com/gemini-hackathon/app/internal/storage"
)

type contextKey string

const (
	userIDKey      contextKey = "userID"
	clerkUserIDKey contextKey = "clerkUserID"
)

// accountDeletionPath is the only route a deleted (tombstoned) Clerk subject
// may reach, so an interrupted deletion can finish its Clerk cleanup.
const accountDeletionPath = "/v1/users/me"

// TokenVerifier validates a Clerk session token and returns its subject.
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (string, error)
}

// UserResolver maps a verified Clerk subject to the local account.
type UserResolver interface {
	Resolve(ctx context.Context, clerkUserID string) (auth.Resolution, error)
}

type AuthMiddleware struct {
	verifier TokenVerifier
	resolver UserResolver
}

func NewAuthMiddleware(verifier TokenVerifier, resolver UserResolver) *AuthMiddleware {
	return &AuthMiddleware{verifier: verifier, resolver: resolver}
}

func (m *AuthMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if token == "" {
			httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized: missing token")
			return
		}

		clerkUserID, err := m.verifier.Verify(r.Context(), token)
		if err != nil {
			if errors.Is(err, auth.ErrVerifierUnavailable) {
				logger.GetDefaultLogger().WithRequestID(GetRequestID(r.Context())).
					ErrorWithErr(err, "Clerk signing keys unavailable")
				httputil.WriteJSONError(w, http.StatusServiceUnavailable, "Authentication service unavailable")
				return
			}
			httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized: invalid token")
			return
		}

		resolution, err := m.resolver.Resolve(r.Context(), clerkUserID)
		if err != nil {
			writeResolveError(w, r, err)
			return
		}

		ctx := context.WithValue(r.Context(), clerkUserIDKey, clerkUserID)
		if resolution.Deleted {
			if r.Method != http.MethodDelete || r.URL.Path != accountDeletionPath {
				httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized: account deleted")
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		ctx = context.WithValue(ctx, userIDKey, resolution.UserID)
		recordRequestUser(ctx, resolution.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeResolveError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrIdentityNotFound):
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized: invalid token")
	case errors.Is(err, auth.ErrVerifiedEmailRequired):
		httputil.WriteJSONError(w, http.StatusForbidden, "Verified email required")
	case errors.Is(err, storage.ErrLegacyLinkRequired):
		httputil.WriteJSONError(w, http.StatusConflict, storage.ErrLegacyLinkRequired.Error())
	case errors.Is(err, auth.ErrIdentityUnavailable):
		logger.GetDefaultLogger().WithRequestID(GetRequestID(r.Context())).
			ErrorWithErr(err, "Clerk identity lookup failed")
		httputil.WriteJSONError(w, http.StatusServiceUnavailable, "Authentication service unavailable")
	default:
		logger.GetDefaultLogger().WithRequestID(GetRequestID(r.Context())).
			ErrorWithErr(err, "Failed to resolve local user")
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Internal server error")
	}
}

func extractBearerToken(r *http.Request) string {
	scheme, token, ok := strings.Cut(strings.TrimSpace(r.Header.Get("Authorization")), " ")
	if !ok || !strings.EqualFold(scheme, "bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

func GetUserID(ctx context.Context) int64 {
	if id, ok := ctx.Value(userIDKey).(int64); ok {
		return id
	}
	return 0
}

func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// GetClerkUserID returns the verified Clerk subject of the request, if any.
func GetClerkUserID(ctx context.Context) string {
	if id, ok := ctx.Value(clerkUserIDKey).(string); ok {
		return id
	}
	return ""
}

// WithClerkUserID is used by handler tests to simulate a verified subject.
func WithClerkUserID(ctx context.Context, clerkUserID string) context.Context {
	return context.WithValue(ctx, clerkUserIDKey, clerkUserID)
}

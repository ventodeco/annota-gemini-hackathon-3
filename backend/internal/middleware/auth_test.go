package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gemini-hackathon/app/internal/auth"
	"github.com/gemini-hackathon/app/internal/httputil"
	"github.com/gemini-hackathon/app/internal/storage"
)

type stubVerifier struct {
	subject string
	err     error
	tokens  []string
}

func (s *stubVerifier) Verify(_ context.Context, token string) (string, error) {
	s.tokens = append(s.tokens, token)
	return s.subject, s.err
}

type stubResolver struct {
	resolution auth.Resolution
	err        error
}

func (s stubResolver) Resolve(context.Context, string) (auth.Resolution, error) {
	return s.resolution, s.err
}

func TestAuthMiddleware(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		headers     map[string]string
		verifier    *stubVerifier
		resolver    stubResolver
		wantStatus  int
		wantMessage string
		wantUserID  int64
	}{
		{
			name:        "missing token",
			verifier:    &stubVerifier{subject: "user_1"},
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "Unauthorized: missing token",
		},
		{
			name:        "legacy x-token header is ignored",
			headers:     map[string]string{"x-token": "legacy"},
			verifier:    &stubVerifier{subject: "user_1"},
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "Unauthorized: missing token",
		},
		{
			name:        "non-bearer scheme",
			headers:     map[string]string{"Authorization": "Basic abc"},
			verifier:    &stubVerifier{subject: "user_1"},
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "Unauthorized: missing token",
		},
		{
			name:        "invalid session",
			headers:     map[string]string{"Authorization": "Bearer bad"},
			verifier:    &stubVerifier{err: auth.ErrInvalidSession},
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "Unauthorized: invalid token",
		},
		{
			name:        "signing keys unavailable",
			headers:     map[string]string{"Authorization": "Bearer t"},
			verifier:    &stubVerifier{err: auth.ErrVerifierUnavailable},
			wantStatus:  http.StatusServiceUnavailable,
			wantMessage: "Authentication service unavailable",
		},
		{
			name:       "valid session resolves numeric user",
			headers:    map[string]string{"Authorization": "bearer t"},
			verifier:   &stubVerifier{subject: "user_1"},
			resolver:   stubResolver{resolution: auth.Resolution{UserID: 42}},
			wantStatus: http.StatusOK,
			wantUserID: 42,
		},
		{
			name:        "clerk user missing",
			headers:     map[string]string{"Authorization": "Bearer t"},
			verifier:    &stubVerifier{subject: "user_1"},
			resolver:    stubResolver{err: auth.ErrIdentityNotFound},
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "Unauthorized: invalid token",
		},
		{
			name:        "verified email required",
			headers:     map[string]string{"Authorization": "Bearer t"},
			verifier:    &stubVerifier{subject: "user_1"},
			resolver:    stubResolver{err: auth.ErrVerifiedEmailRequired},
			wantStatus:  http.StatusForbidden,
			wantMessage: "Verified email required",
		},
		{
			name:        "legacy google link required",
			headers:     map[string]string{"Authorization": "Bearer t"},
			verifier:    &stubVerifier{subject: "user_1"},
			resolver:    stubResolver{err: storage.ErrLegacyLinkRequired},
			wantStatus:  http.StatusConflict,
			wantMessage: "legacy_google_link_required",
		},
		{
			name:        "clerk outage on first request",
			headers:     map[string]string{"Authorization": "Bearer t"},
			verifier:    &stubVerifier{subject: "user_1"},
			resolver:    stubResolver{err: auth.ErrIdentityUnavailable},
			wantStatus:  http.StatusServiceUnavailable,
			wantMessage: "Authentication service unavailable",
		},
		{
			name:        "database failure",
			headers:     map[string]string{"Authorization": "Bearer t"},
			verifier:    &stubVerifier{subject: "user_1"},
			resolver:    stubResolver{err: errors.New("db down")},
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "Internal server error",
		},
		{
			name:        "deleted subject cannot read data",
			headers:     map[string]string{"Authorization": "Bearer t"},
			verifier:    &stubVerifier{subject: "user_1"},
			resolver:    stubResolver{resolution: auth.Resolution{Deleted: true}},
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "Unauthorized: account deleted",
		},
		{
			name:        "deleted subject cannot GET profile",
			path:        "/v1/users/me",
			headers:     map[string]string{"Authorization": "Bearer t"},
			verifier:    &stubVerifier{subject: "user_1"},
			resolver:    stubResolver{resolution: auth.Resolution{Deleted: true}},
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "Unauthorized: account deleted",
		},
		{
			name:       "deleted subject may retry account deletion",
			method:     http.MethodDelete,
			path:       "/v1/users/me",
			headers:    map[string]string{"Authorization": "Bearer t"},
			verifier:   &stubVerifier{subject: "user_1"},
			resolver:   stubResolver{resolution: auth.Resolution{Deleted: true}},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method := tt.method
			if method == "" {
				method = http.MethodGet
			}
			path := tt.path
			if path == "" {
				path = "/v1/scans"
			}
			var gotUserID int64
			var gotClerkID string
			handler := NewAuthMiddleware(tt.verifier, tt.resolver).Handle(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotUserID = GetUserID(r.Context())
				gotClerkID = GetClerkUserID(r.Context())
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(method, path, nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantStatus != http.StatusOK {
				if got := rec.Header().Get("Content-Type"); got != "application/json" {
					t.Fatalf("expected JSON content type, got %q", got)
				}
				var body httputil.ErrorResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatalf("decode error body: %v", err)
				}
				if body.Message != tt.wantMessage {
					t.Fatalf("message = %q, want %q", body.Message, tt.wantMessage)
				}
				return
			}
			if gotUserID != tt.wantUserID {
				t.Fatalf("user id = %d, want %d", gotUserID, tt.wantUserID)
			}
			if gotClerkID != "user_1" {
				t.Fatalf("clerk id = %q, want user_1", gotClerkID)
			}
		})
	}
}

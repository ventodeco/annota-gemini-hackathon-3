package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/storage"
)

type sessionKey string

const sessionIDKey sessionKey = "sessionID"

type SessionMiddleware struct {
	db         storage.DB
	cookieName string
	secure     bool
}

func NewSessionMiddleware(db storage.DB, cookieName string, secure bool) *SessionMiddleware {
	return &SessionMiddleware{
		db:         db,
		cookieName: cookieName,
		secure:     secure,
	}
}

func (s *SessionMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionID := s.getOrCreateSession(r, w)
		ctx := context.WithValue(r.Context(), sessionIDKey, sessionID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *SessionMiddleware) getOrCreateSession(r *http.Request, w http.ResponseWriter) string {
	cookie, err := r.Cookie(s.cookieName)
	if err == nil && cookie.Value != "" {
		session, err := s.db.GetSession(r.Context(), cookie.Value)
		if err == nil && session != nil {
			s.db.UpdateSessionLastSeen(r.Context(), cookie.Value)
			return cookie.Value
		}
	}

	sessionID := generateSessionID()
	now := time.Now()
	session := &models.Session{
		ID:         sessionID,
		UserID:     nil,
		CreatedAt:  now,
		LastSeenAt: now,
	}

	if err := s.db.CreateSession(r.Context(), session); err != nil {
		return ""
	}

	http.SetCookie(w, &http.Cookie{
		Name:     s.cookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   86400 * 365,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})

	return sessionID
}

func generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func GetSessionID(ctx context.Context) string {
	if id, ok := ctx.Value(sessionIDKey).(string); ok {
		return id
	}
	return ""
}

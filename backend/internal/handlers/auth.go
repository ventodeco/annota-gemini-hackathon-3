package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gemini-hackathon/app/internal/auth"
	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/logger"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/storage"
)

type AuthHandlers struct {
	googleOAuth  *auth.GoogleOAuthService
	tokenService *auth.TokenService
	db           storage.DB
	config       *config.Config
}

func NewAuthHandlers(googleOAuth *auth.GoogleOAuthService, tokenService *auth.TokenService, db storage.DB, cfg *config.Config) *AuthHandlers {
	return &AuthHandlers{
		googleOAuth:  googleOAuth,
		tokenService: tokenService,
		db:           db,
		config:       cfg,
	}
}

type GoogleStateResponse struct {
	SSORedirection string `json:"ssoRedirection"`
}

type GoogleCallbackRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

type GoogleCallbackResponse struct {
	Token         string `json:"token"`
	ExpirySeconds int    `json:"expirySeconds"`
	ExpiresAt     string `json:"expiresAt"`
}

func (h *AuthHandlers) GoogleStateAPI(w http.ResponseWriter, r *http.Request) {
	log := logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context()))

	if r.Method != http.MethodGet {
		log.Warnf("Method not allowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state := generateState(32)
	redirectURL := h.googleOAuth.GetAuthURL(state)

	if err := h.googleOAuth.RedisClient().SetState(r.Context(), state, "", 10*time.Minute); err != nil {
		log.ErrorWithErr(err, "Failed to store state in Redis")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Infof("Generated OAuth state for Google SSO")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GoogleStateResponse{
		SSORedirection: redirectURL,
	})
}

func (h *AuthHandlers) GoogleCallbackRedirect(w http.ResponseWriter, r *http.Request) {
	log := logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context()))

	if r.Method != http.MethodGet {
		log.Warnf("Method not allowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	if state == "" || code == "" {
		log.Warn("Missing state or code in OAuth callback")
		http.Error(w, "Missing state or code", http.StatusBadRequest)
		return
	}

	statePreview := state
	if len(state) > 8 {
		statePreview = state[:8]
	}
	log.Infof("Processing OAuth callback with state: %s...", statePreview)

	redirectURL := fmt.Sprintf("%s/auth/callback?state=%s&code=%s", h.config.AppBaseURL, state, code)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *AuthHandlers) GoogleCallbackAPI(w http.ResponseWriter, r *http.Request) {
	log := logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context()))

	if r.Method != http.MethodPost {
		log.Warnf("Method not allowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req GoogleCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warnf("Failed to decode request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Code == "" {
		log.Warn("Empty authorization code received")
		http.Error(w, "Code is required", http.StatusBadRequest)
		return
	}

	if req.State == "" {
		log.Warn("Empty state received")
		http.Error(w, "State is required", http.StatusBadRequest)
		return
	}

	log.Infof("Attempting to exchange authorization code")

	_, err := h.googleOAuth.RedisClient().GetState(r.Context(), req.State)
	if err != nil {
		log.ErrorWithErr(err, "State validation failed")
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	oauthToken, err := h.googleOAuth.ExchangeCode(r.Context(), req.Code)
	if err != nil {
		log.ErrorWithErr(err, "Failed to exchange code with Google")
		http.Error(w, "Failed to exchange code", http.StatusInternalServerError)
		return
	}

	userInfo, err := h.googleOAuth.GetUserInfo(r.Context(), oauthToken)
	if err != nil {
		log.ErrorWithErr(err, "Failed to get user info from Google")
		http.Error(w, "Failed to get user info", http.StatusInternalServerError)
		return
	}

	log.Infof("Retrieved user info from Google: %s", userInfo.Email)

	user, err := h.db.GetUserByProvider(r.Context(), "google", userInfo.ID)
	if err != nil {
		log.ErrorWithErr(err, "Failed to get user by provider from database")
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	isNewUser := false
	if user == nil {
		user = &models.User{
			Email:             userInfo.Email,
			Provider:          "google",
			ProviderID:        userInfo.ID,
			PreferredLanguage: "ID",
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		if userInfo.PictureURL != "" {
			user.AvatarURL = &userInfo.PictureURL
		}

		if err := h.db.CreateUser(r.Context(), user); err != nil {
			log.ErrorWithErr(err, "Failed to create new user in database")
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}
		isNewUser = true
		log.Infof("Created new user: %s (ID: %d)", user.Email, user.ID)
	} else {
		log.Infof("Existing user authenticated: %s (ID: %d)", user.Email, user.ID)
	}

	token, expiresAt, err := h.tokenService.GenerateToken(user.ID)
	if err != nil {
		log.ErrorWithErr(err, "Failed to generate JWT token")
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	log.WithFields(map[string]any{
		"user_id":          user.ID,
		"is_new_user":      isNewUser,
		"token_expiry_sec": h.config.TokenExpiryMinutes * 60,
	}).Infof("Successfully generated JWT token for user")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GoogleCallbackResponse{
		Token:         token,
		ExpirySeconds: int(h.config.TokenExpiryMinutes * 60),
		ExpiresAt:     expiresAt.Format(time.RFC3339),
	})
}

func (h *AuthHandlers) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.GoogleCallbackRedirect(w, r)
	case http.MethodPost:
		h.GoogleCallbackAPI(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func generateState(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateCallbackState(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func buildCallbackURL(baseURL, state, code string) string {
	params := url.Values{}
	params.Set("state", state)
	params.Set("code", code)
	return fmt.Sprintf("%s/auth/callback?%s", baseURL, params.Encode())
}

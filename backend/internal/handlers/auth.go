package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gemini-hackathon/app/internal/auth"
	"github.com/gemini-hackathon/app/internal/config"
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
	Code string `json:"code"`
}

type GoogleCallbackResponse struct {
	Token         string `json:"token"`
	ExpirySeconds int    `json:"expirySeconds"`
	ExpiresAt     string `json:"expiresAt"`
}

func (h *AuthHandlers) GoogleStateAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state := generateState(32)
	redirectURL := h.googleOAuth.GetAuthURL(state)

	if err := h.googleOAuth.RedisClient().SetState(r.Context(), state, "", 10*time.Minute); err != nil {
		log.Printf("Failed to store state in Redis: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GoogleStateResponse{
		SSORedirection: redirectURL,
	})
}

func (h *AuthHandlers) GoogleCallbackRedirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	if state == "" || code == "" {
		http.Error(w, "Missing state or code", http.StatusBadRequest)
		return
	}

	redirectURL := fmt.Sprintf("%s/auth/callback?state=%s&code=%s", h.config.AppBaseURL, state, code)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *AuthHandlers) GoogleCallbackAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req GoogleCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Code == "" {
		http.Error(w, "Code is required", http.StatusBadRequest)
		return
	}

	_, err := h.googleOAuth.RedisClient().GetState(r.Context(), req.Code)
	if err != nil {
		log.Printf("State validation failed: %v", err)
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	oauthToken, err := h.googleOAuth.ExchangeCode(r.Context(), req.Code)
	if err != nil {
		log.Printf("Failed to exchange code: %v", err)
		http.Error(w, "Failed to exchange code", http.StatusInternalServerError)
		return
	}

	userInfo, err := h.googleOAuth.GetUserInfo(r.Context(), oauthToken)
	if err != nil {
		log.Printf("Failed to get user info: %v", err)
		http.Error(w, "Failed to get user info", http.StatusInternalServerError)
		return
	}

	user, err := h.db.GetUserByProvider(r.Context(), "google", userInfo.ID)
	if err != nil {
		log.Printf("Failed to get user by provider: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

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
			log.Printf("Failed to create user: %v", err)
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}
	}

	token, expiresAt, err := h.tokenService.GenerateToken(user.ID)
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GoogleCallbackResponse{
		Token:         token,
		ExpirySeconds: int(h.config.TokenExpiryMinutes * 60),
		ExpiresAt:     expiresAt.Format(time.RFC3339),
	})
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

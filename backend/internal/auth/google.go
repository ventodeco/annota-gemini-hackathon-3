package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/storage"
)

type GoogleOAuthService struct {
	config      *oauth2.Config
	redisClient storage.RedisClient
	appBaseURL  string
}

type UserInfo struct {
	Email      string
	ID         string
	Name       string
	PictureURL string
}

func NewGoogleOAuthService(cfg *config.Config, redis storage.RedisClient) *GoogleOAuthService {
	oauthConfig := &oauth2.Config{
		ClientID:     cfg.GoogleOAuthClientID,
		ClientSecret: cfg.GoogleOAuthClientSecret,
		RedirectURL:  fmt.Sprintf("%s/v1/auth/google/callback", cfg.AppBaseURL),
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	return &GoogleOAuthService{
		config:      oauthConfig,
		redisClient: redis,
		appBaseURL:  cfg.AppBaseURL,
	}
}

func (s *GoogleOAuthService) GetAuthURL(state string, redirectURI string) string {
	if redirectURI != "" {
		return s.config.AuthCodeURL(state,
			oauth2.SetAuthURLParam("redirect_uri", redirectURI),
			oauth2.SetAuthURLParam("prompt", "consent"))
	}
	return s.config.AuthCodeURL(state, oauth2.SetAuthURLParam("prompt", "consent"))
}

func (s *GoogleOAuthService) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	return s.config.Exchange(ctx, code)
}

func (s *GoogleOAuthService) GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	client := s.config.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google user info API returned status %d", resp.StatusCode)
	}

	var data struct {
		Email     string `json:"email"`
		ID        string `json:"id"`
		Name      string `json:"name"`
		Picture   string `json:"picture"`
		LocalPart string `json:"local_part"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	if data.Email == "" && data.LocalPart != "" {
		data.Email = data.LocalPart + "@gmail.com"
	}

	return &UserInfo{
		Email:      data.Email,
		ID:         data.ID,
		Name:       data.Name,
		PictureURL: strings.ReplaceAll(data.Picture, "=s96-c", "=s200-c"),
	}, nil
}

func (s *GoogleOAuthService) RedisClient() storage.RedisClient {
	return s.redisClient
}

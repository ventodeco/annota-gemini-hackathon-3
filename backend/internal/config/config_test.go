package config

import (
	"reflect"
	"testing"
)

func TestConfig_GetAllowedOriginsList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty uses defaults",
			input:    "",
			expected: []string{"http://localhost:5173", "http://localhost:3000"},
		},
		{
			name:     "single origin",
			input:    "https://app.example.com",
			expected: []string{"https://app.example.com"},
		},
		{
			name:     "multiple origins",
			input:    "https://a.com,https://b.com",
			expected: []string{"https://a.com", "https://b.com"},
		},
		{
			name:     "origins with spaces",
			input:    "https://a.com, https://b.com , https://c.com",
			expected: []string{"https://a.com", "https://b.com", "https://c.com"},
		},
		{
			name:     "wildcard",
			input:    "*",
			expected: []string{"*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{AllowedOrigins: tt.input}
			got := cfg.GetAllowedOriginsList()
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("GetAllowedOriginsList() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConfig_IsOriginAllowed(t *testing.T) {
	tests := []struct {
		name           string
		allowedOrigins string
		origin         string
		want           bool
	}{
		{
			name:           "wildcard allows any origin",
			allowedOrigins: "*",
			origin:         "https://any-domain.com",
			want:           true,
		},
		{
			name:           "allowed origin in list",
			allowedOrigins: "http://localhost:5173,http://localhost:3000",
			origin:         "http://localhost:5173",
			want:           true,
		},
		{
			name:           "disallowed origin not in list",
			allowedOrigins: "http://localhost:5173",
			origin:         "http://localhost:3000",
			want:           false,
		},
		{
			name:           "empty origin not allowed",
			allowedOrigins: "http://localhost:5173",
			origin:         "",
			want:           false,
		},
		{
			name:           "origin with different scheme",
			allowedOrigins: "https://app.com",
			origin:         "http://app.com",
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{AllowedOrigins: tt.allowedOrigins}
			if got := cfg.IsOriginAllowed(tt.origin); got != tt.want {
				t.Errorf("IsOriginAllowed(%q) = %v, want %v", tt.origin, got, tt.want)
			}
		})
	}
}

func TestLoadReadsRateLimitConfig(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "openrouter-key")
	t.Setenv("MINIMAX_API_KEY", "minimax-key")
	t.Setenv("DB_CONNECTION_STRING", "postgres://test")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678912")
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "test-client-secret")
	t.Setenv("AI_RATE_LIMIT", "7")
	t.Setenv("AI_RATE_LIMIT_WINDOW_SECONDS", "30")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.AIRateLimit != 7 {
		t.Fatalf("AIRateLimit = %d, want 7", cfg.AIRateLimit)
	}
	if cfg.AIRateLimitWindowSeconds != 30 {
		t.Fatalf("AIRateLimitWindowSeconds = %d, want 30", cfg.AIRateLimitWindowSeconds)
	}
}

func TestLoadReadsAIProviderConfig(t *testing.T) {
	t.Setenv("AI_PROVIDER", "minimax")
	t.Setenv("OPENROUTER_API_KEY", "openrouter-key")
	t.Setenv("MINIMAX_API_KEY", "minimax-key")
	t.Setenv("GEMINI_API_KEY", "gemini-key")
	t.Setenv("OPENROUTER_BASE_URL", "https://openrouter.test/api/v1")
	t.Setenv("OPENROUTER_OCR_MODEL", "custom-ocr")
	t.Setenv("MINIMAX_ANTHROPIC_BASE_URL", "https://minimax.test/anthropic/v1")
	t.Setenv("MINIMAX_TEXT_MODEL", "MiniMax-M2.7-highspeed")
	t.Setenv("MINIMAX_TTS_BASE_URL", "https://minimax.test/v1")
	t.Setenv("MINIMAX_TTS_MODEL", "speech-2.8-hd")
	t.Setenv("MINIMAX_TTS_VOICE_ID", "Japanese_Whisper_Belle")
	t.Setenv("DB_CONNECTION_STRING", "postgres://test")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678912")
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "test-client-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.OpenRouterAPIKey != "openrouter-key" {
		t.Fatalf("OpenRouterAPIKey = %q, want openrouter-key", cfg.OpenRouterAPIKey)
	}
	if cfg.AIProvider != "minimax" {
		t.Fatalf("AIProvider = %q, want minimax", cfg.AIProvider)
	}
	if cfg.GeminiAPIKey != "gemini-key" {
		t.Fatalf("GeminiAPIKey = %q, want gemini-key", cfg.GeminiAPIKey)
	}
	if cfg.MiniMaxAPIKey != "minimax-key" {
		t.Fatalf("MiniMaxAPIKey = %q, want minimax-key", cfg.MiniMaxAPIKey)
	}
	if cfg.OpenRouterBaseURL != "https://openrouter.test/api/v1" {
		t.Fatalf("OpenRouterBaseURL = %q", cfg.OpenRouterBaseURL)
	}
	if cfg.OpenRouterOCRModel != "custom-ocr" {
		t.Fatalf("OpenRouterOCRModel = %q", cfg.OpenRouterOCRModel)
	}
	if cfg.MiniMaxAnthropicBaseURL != "https://minimax.test/anthropic/v1" {
		t.Fatalf("MiniMaxAnthropicBaseURL = %q", cfg.MiniMaxAnthropicBaseURL)
	}
	if cfg.MiniMaxTextModel != "MiniMax-M2.7-highspeed" {
		t.Fatalf("MiniMaxTextModel = %q", cfg.MiniMaxTextModel)
	}
	if cfg.MiniMaxTTSBaseURL != "https://minimax.test/v1" {
		t.Fatalf("MiniMaxTTSBaseURL = %q", cfg.MiniMaxTTSBaseURL)
	}
	if cfg.MiniMaxTTSModel != "speech-2.8-hd" {
		t.Fatalf("MiniMaxTTSModel = %q", cfg.MiniMaxTTSModel)
	}
	if cfg.MiniMaxTTSVoiceID != "Japanese_Whisper_Belle" {
		t.Fatalf("MiniMaxTTSVoiceID = %q", cfg.MiniMaxTTSVoiceID)
	}
}

func TestLoadUsesAIProviderDefaults(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "openrouter-key")
	t.Setenv("MINIMAX_API_KEY", "minimax-key")
	t.Setenv("DB_CONNECTION_STRING", "postgres://test")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678912")
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "test-client-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.OpenRouterBaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("OpenRouterBaseURL = %q", cfg.OpenRouterBaseURL)
	}
	if cfg.OpenRouterOCRModel != "baidu/qianfan-ocr-fast:free" {
		t.Fatalf("OpenRouterOCRModel = %q", cfg.OpenRouterOCRModel)
	}
	if cfg.MiniMaxAnthropicBaseURL != "https://api.minimax.io/anthropic/v1" {
		t.Fatalf("MiniMaxAnthropicBaseURL = %q", cfg.MiniMaxAnthropicBaseURL)
	}
	if cfg.MiniMaxTextModel != "MiniMax-M2.7" {
		t.Fatalf("MiniMaxTextModel = %q", cfg.MiniMaxTextModel)
	}
	if cfg.MiniMaxTTSBaseURL != "https://api-uw.minimax.io/v1" {
		t.Fatalf("MiniMaxTTSBaseURL = %q", cfg.MiniMaxTTSBaseURL)
	}
	if cfg.MiniMaxTTSModel != "speech-2.8-hd" {
		t.Fatalf("MiniMaxTTSModel = %q", cfg.MiniMaxTTSModel)
	}
	if cfg.AIProvider != "minimax" {
		t.Fatalf("AIProvider = %q, want minimax", cfg.AIProvider)
	}
}

func TestLoadReadsGeminiProviderConfig(t *testing.T) {
	t.Setenv("AI_PROVIDER", "gemini")
	t.Setenv("GEMINI_API_KEY", "gemini-key")
	t.Setenv("DB_CONNECTION_STRING", "postgres://test")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678912")
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "test-client-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.AIProvider != "gemini" {
		t.Fatalf("AIProvider = %q, want gemini", cfg.AIProvider)
	}
	if cfg.GeminiAPIKey != "gemini-key" {
		t.Fatalf("GeminiAPIKey = %q, want gemini-key", cfg.GeminiAPIKey)
	}
}

func TestLoadUsesGoogleAPIKeyAsGeminiFallback(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "google-key")
	t.Setenv("GEMINI_API_KEY", "")

	if got := getGeminiAPIKey(); got != "google-key" {
		t.Fatalf("getGeminiAPIKey() = %q, want google-key", got)
	}
}

func TestConfigValidateRequiresGeminiKeyInGeminiMode(t *testing.T) {
	cfg := validConfig()
	cfg.AIProvider = "gemini"
	cfg.GeminiAPIKey = ""
	cfg.OpenRouterAPIKey = ""
	cfg.MiniMaxAPIKey = ""

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected missing Gemini key to fail validation")
	}
}

func TestConfigValidateAllowsGeminiModeWithoutMiniMaxStack(t *testing.T) {
	cfg := validConfig()
	cfg.AIProvider = "gemini"
	cfg.GeminiAPIKey = "gemini-key"
	cfg.OpenRouterAPIKey = ""
	cfg.MiniMaxAPIKey = ""

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestConfigValidateRejectsUnknownAIProvider(t *testing.T) {
	cfg := validConfig()
	cfg.AIProvider = "unknown"

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected unknown AI_PROVIDER to fail validation")
	}
}

func TestConfigValidateRequiresStrongJWTSecret(t *testing.T) {
	cfg := &Config{
		OpenRouterAPIKey:         "openrouter-key",
		MiniMaxAPIKey:            "minimax-key",
		DBConnectionString:       "postgres://test",
		UploadDir:                "data/uploads",
		MaxUploadSize:            1,
		FrontendBaseURL:          "http://localhost:5173",
		TokenExpiryMinutes:       30,
		DefaultPageSize:          20,
		JWTSecret:                "short",
		GoogleOAuthClientID:      "client-id",
		GoogleOAuthClientSecret:  "client-secret",
		AIRateLimit:              60,
		AIRateLimitWindowSeconds: 3600,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected weak JWT_SECRET to fail validation")
	}
}

func validConfig() *Config {
	return &Config{
		AIProvider:               "minimax",
		OpenRouterAPIKey:         "openrouter-key",
		OpenRouterBaseURL:        "https://openrouter.test/api/v1",
		OpenRouterOCRModel:       "ocr-model",
		MiniMaxAPIKey:            "minimax-key",
		MiniMaxAnthropicBaseURL:  "https://minimax.test/anthropic/v1",
		MiniMaxTextModel:         "text-model",
		MiniMaxTTSBaseURL:        "https://minimax.test/v1",
		MiniMaxTTSModel:          "tts-model",
		MiniMaxTTSVoiceID:        "voice-id",
		DBConnectionString:       "postgres://test",
		UploadDir:                "data/uploads",
		MaxUploadSize:            1,
		FrontendBaseURL:          "http://localhost:5173",
		TokenExpiryMinutes:       30,
		DefaultPageSize:          20,
		JWTSecret:                "01234567890123456789012345678912",
		GoogleOAuthClientID:      "client-id",
		GoogleOAuthClientSecret:  "client-secret",
		AIRateLimit:              60,
		AIRateLimitWindowSeconds: 3600,
	}
}

func TestConfigValidateRequiresGoogleOAuthClientID(t *testing.T) {
	cfg := &Config{
		OpenRouterAPIKey:         "openrouter-key",
		MiniMaxAPIKey:            "minimax-key",
		DBConnectionString:       "postgres://test",
		UploadDir:                "data/uploads",
		MaxUploadSize:            1,
		FrontendBaseURL:          "http://localhost:5173",
		TokenExpiryMinutes:       30,
		DefaultPageSize:          20,
		JWTSecret:                "01234567890123456789012345678912",
		GoogleOAuthClientID:      "",
		GoogleOAuthClientSecret:  "some-secret",
		AIRateLimit:              60,
		AIRateLimitWindowSeconds: 3600,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected missing GOOGLE_OAUTH_CLIENT_ID to fail validation")
	}
}

func TestConfigValidateRequiresGoogleOAuthClientSecret(t *testing.T) {
	cfg := &Config{
		OpenRouterAPIKey:         "openrouter-key",
		MiniMaxAPIKey:            "minimax-key",
		DBConnectionString:       "postgres://test",
		UploadDir:                "data/uploads",
		MaxUploadSize:            1,
		FrontendBaseURL:          "http://localhost:5173",
		TokenExpiryMinutes:       30,
		DefaultPageSize:          20,
		JWTSecret:                "01234567890123456789012345678912",
		GoogleOAuthClientID:      "client-id",
		GoogleOAuthClientSecret:  "",
		AIRateLimit:              60,
		AIRateLimitWindowSeconds: 3600,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected missing GOOGLE_OAUTH_CLIENT_SECRET to fail validation")
	}
}

func TestConfigValidateRejectsWildcardProductionOrigins(t *testing.T) {
	cfg := &Config{
		OpenRouterAPIKey:         "openrouter-key",
		MiniMaxAPIKey:            "minimax-key",
		DBConnectionString:       "postgres://test",
		UploadDir:                "data/uploads",
		MaxUploadSize:            1,
		FrontendBaseURL:          "https://annota.example.com",
		TokenExpiryMinutes:       30,
		DefaultPageSize:          20,
		JWTSecret:                "01234567890123456789012345678912",
		GoogleOAuthClientID:      "client-id",
		GoogleOAuthClientSecret:  "client-secret",
		AllowedOrigins:           "*",
		AppEnv:                   "production",
		AIRateLimit:              60,
		AIRateLimitWindowSeconds: 3600,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected wildcard ALLOWED_ORIGINS to fail in production")
	}
}

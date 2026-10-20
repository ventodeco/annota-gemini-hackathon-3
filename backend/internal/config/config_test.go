package config

import (
	"os"
	"path/filepath"
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
	setClerkTestEnv(t)
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
	setClerkTestEnv(t)

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
	setClerkTestEnv(t)

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
	setClerkTestEnv(t)

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
		DefaultPageSize:          20,
		ClerkSecretKey:           "sk_test_fixture",
		ClerkPublishableKey:      testPublishableKey,
		ClerkAuthorizedParties:   []string{"http://localhost:5173"},
		AIRateLimit:              60,
		AIRateLimitWindowSeconds: 3600,
	}
}

// testPublishableKey encodes the fixture host annota-test.clerk.accounts.dev.
const testPublishableKey = "pk_test_YW5ub3RhLXRlc3QuY2xlcmsuYWNjb3VudHMuZGV2JA=="

func setClerkTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CLERK_SECRET_KEY", "sk_test_fixture")
	t.Setenv("CLERK_PUBLISHABLE_KEY", testPublishableKey)
	t.Setenv("CLERK_AUTHORIZED_PARTIES", "")
	t.Setenv("FRONTEND_BASE_URL", "http://localhost:5173")
}

func TestConfigValidateClerkSettings(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{name: "valid", mutate: func(*Config) {}},
		{name: "missing secret key", mutate: func(c *Config) { c.ClerkSecretKey = "" }, wantErr: true},
		{name: "publishable key in secret slot", mutate: func(c *Config) { c.ClerkSecretKey = testPublishableKey }, wantErr: true},
		{name: "missing publishable key", mutate: func(c *Config) { c.ClerkPublishableKey = "" }, wantErr: true},
		{name: "malformed publishable key", mutate: func(c *Config) { c.ClerkPublishableKey = "pk_test_not-base64!" }, wantErr: true},
		{name: "no authorized parties", mutate: func(c *Config) { c.ClerkAuthorizedParties = nil }, wantErr: true},
		{name: "wildcard authorized party", mutate: func(c *Config) { c.ClerkAuthorizedParties = []string{"*"} }, wantErr: true},
		{name: "authorized party with path", mutate: func(c *Config) { c.ClerkAuthorizedParties = []string{"https://annota.example.com/app"} }, wantErr: true},
		{name: "multiple origins", mutate: func(c *Config) {
			c.ClerkAuthorizedParties = []string{"https://annota.example.com", "http://localhost:5173"}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(cfg)
			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestConfigValidateDerivesClerkIssuer(t *testing.T) {
	cfg := validConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.ClerkIssuer != "https://annota-test.clerk.accounts.dev" {
		t.Fatalf("unexpected issuer %q", cfg.ClerkIssuer)
	}
}

func TestLoadDefaultsAuthorizedPartiesToFrontendBaseURL(t *testing.T) {
	t.Setenv("AI_PROVIDER", "gemini")
	t.Setenv("GEMINI_API_KEY", "gemini-key")
	t.Setenv("AI_ROUTING_CONFIG", "")
	t.Setenv("DB_CONNECTION_STRING", "postgres://test")
	setClerkTestEnv(t)
	t.Setenv("FRONTEND_BASE_URL", "https://annota.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.ClerkAuthorizedParties) != 1 || cfg.ClerkAuthorizedParties[0] != "https://annota.example.com" {
		t.Fatalf("unexpected authorized parties %v", cfg.ClerkAuthorizedParties)
	}
}

func TestConfigValidateRejectsWildcardProductionOrigins(t *testing.T) {
	cfg := validConfig()
	cfg.AllowedOrigins = "*"
	cfg.AppEnv = "production"

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected wildcard ALLOWED_ORIGINS to fail in production")
	}
}

func TestLoadRoutedConfigOnlyRequiresActiveKeys(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("AI_PROVIDER", "routed")
	// Synthetic values never trigger provider calls.
	data := []byte(`{"version":1,"profiles":{"gateway":{"kind":"openrouter","region":"global","base_url":"https://openrouter.test/api/v1","api_key_env":"ROUTED_TEST_KEY","models":{"vision":{"image_input":true,"text_input":true,"text_output":true,"json_mode":"prompt","ocr_output":"json","max_output_tokens":2048},"speech":{"text_input":true,"audio_output":true,"japanese_voices":["ja-test"],"audio_formats":["mp3"]}}}},"tasks":{"ocr":{"profile":"gateway","model":"vision"},"annotation":{"profile":"gateway","model":"vision"},"speech":{"profile":"gateway","model":"speech","voice":"ja-test","format":"mp3"}}}`)
	filename := filepath.Join(t.TempDir(), "routing.json")
	if err := os.WriteFile(filename, data, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AI_ROUTING_CONFIG", filename)
	t.Setenv("ROUTED_TEST_KEY", "fixture-key")
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("MINIMAX_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("DB_CONNECTION_STRING", "postgres://test")
	setClerkTestEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AIRouting == nil || cfg.AIRouting.Profiles["gateway"].APIKey != "fixture-key" {
		t.Fatal("routes not resolved")
	}
	t.Setenv("ROUTED_TEST_KEY", "")
	if _, err := Load(); err == nil {
		t.Fatal("missing active credential accepted")
	}
	t.Setenv("AI_ROUTING_CONFIG", filepath.Join(t.TempDir(), "missing.json"))
	if _, err := Load(); err == nil {
		t.Fatal("missing routing file accepted")
	}
	t.Setenv("AI_ROUTING_CONFIG", "")
	if _, err := Load(); err == nil {
		t.Fatal("missing path accepted")
	}
}

func TestConfigRejectsRoutingFileInLegacyMode(t *testing.T) {
	cfg := validConfig()
	cfg.AIRoutingConfigPath = "routes.json"
	if cfg.Validate() == nil {
		t.Fatal("legacy mode silently ignored routing file")
	}
}

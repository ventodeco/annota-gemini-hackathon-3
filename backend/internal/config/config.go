package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	AIProviderGemini  = "gemini"
	AIProviderMiniMax = "minimax"
)

type Config struct {
	AIProvider              string
	GeminiAPIKey            string
	OpenRouterAPIKey        string
	OpenRouterBaseURL       string
	OpenRouterOCRModel      string
	MiniMaxAPIKey           string
	MiniMaxAnthropicBaseURL string
	MiniMaxTextModel        string
	MiniMaxTTSBaseURL       string
	MiniMaxTTSModel         string
	MiniMaxTTSVoiceID       string
	AppBaseURL              string
	AppEnv                  string
	FrontendBaseURL         string
	Port                    string
	DBConnectionString      string
	UploadDir               string
	MaxUploadSize           int64
	SessionCookieName       string
	SessionSecure           bool

	GoogleOAuthClientID      string
	GoogleOAuthClientSecret  string
	RedisAddr                string
	JWTSecret                string
	TokenExpiryMinutes       int
	DefaultPageSize          int
	KnowledgeCSVPath         string
	AllowedOrigins           string // comma-separated list, * for wildcard
	AIRateLimit              int
	AIRateLimitWindowSeconds int
}

func Load() (*Config, error) {
	// Try loading from current directory and parent directory (project root)
	loadEnvFile(".env.local")
	loadEnvFile("../.env.local")
	loadEnvFile(".env")
	loadEnvFile("../.env")

	// Build PostgreSQL connection string if individual components are provided
	dbConnStr := os.Getenv("DB_CONNECTION_STRING")
	if dbConnStr == "" {
		// Build from individual components
		host := getEnvOrDefault("POSTGRES_HOST", "localhost")
		port := getEnvOrDefault("POSTGRES_PORT", "5432")
		user := getEnvOrDefault("POSTGRES_USER", "gemini_user")
		password := getEnvOrDefault("POSTGRES_PASSWORD", "gemini_password")
		dbname := getEnvOrDefault("POSTGRES_DB", "gemini_db")
		sslmode := getEnvOrDefault("POSTGRES_SSLMODE", "disable")

		if host != "" && port != "" && user != "" && password != "" && dbname != "" {
			dbConnStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
				host, port, user, password, dbname, sslmode)
		}
	}

	appBaseURL := getEnvOrDefault("APP_BASE_URL", "http://localhost:8080")
	frontendBaseURL := getEnvOrDefault("FRONTEND_BASE_URL", appBaseURL)

	cfg := &Config{
		AIProvider:               strings.ToLower(getEnvOrDefault("AI_PROVIDER", AIProviderMiniMax)),
		GeminiAPIKey:             getGeminiAPIKey(),
		OpenRouterAPIKey:         os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterBaseURL:        getEnvOrDefault("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"),
		OpenRouterOCRModel:       getEnvOrDefault("OPENROUTER_OCR_MODEL", "baidu/qianfan-ocr-fast:free"),
		MiniMaxAPIKey:            os.Getenv("MINIMAX_API_KEY"),
		MiniMaxAnthropicBaseURL:  getEnvOrDefault("MINIMAX_ANTHROPIC_BASE_URL", "https://api.minimax.io/anthropic/v1"),
		MiniMaxTextModel:         getEnvOrDefault("MINIMAX_TEXT_MODEL", "MiniMax-M2.7"),
		MiniMaxTTSBaseURL:        getEnvOrDefault("MINIMAX_TTS_BASE_URL", "https://api-uw.minimax.io/v1"),
		MiniMaxTTSModel:          getEnvOrDefault("MINIMAX_TTS_MODEL", "speech-2.8-hd"),
		MiniMaxTTSVoiceID:        getEnvOrDefault("MINIMAX_TTS_VOICE_ID", "Japanese_Whisper_Belle"),
		AppBaseURL:               appBaseURL,
		AppEnv:                   strings.ToLower(getEnvOrDefault("APP_ENV", "development")),
		FrontendBaseURL:          frontendBaseURL,
		Port:                     getEnvOrDefault("PORT", "8080"),
		DBConnectionString:       dbConnStr,
		UploadDir:                getEnvOrDefault("UPLOAD_DIR", "data/uploads"),
		MaxUploadSize:            getEnvAsInt64OrDefault("MAX_UPLOAD_SIZE", 10*1024*1024),
		SessionCookieName:        getEnvOrDefault("SESSION_COOKIE_NAME", "sid"),
		SessionSecure:            getEnvAsBoolOrDefault("SESSION_SECURE", false),
		GoogleOAuthClientID:      os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		GoogleOAuthClientSecret:  os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
		RedisAddr:                getEnvOrDefault("REDIS_ADDR", "localhost:6379"),
		JWTSecret:                os.Getenv("JWT_SECRET"),
		TokenExpiryMinutes:       getEnvAsIntOrDefault("TOKEN_EXPIRY_MINUTES", 30),
		DefaultPageSize:          getEnvAsIntOrDefault("DEFAULT_PAGE_SIZE", 20),
		KnowledgeCSVPath:         getEnvOrDefault("KNOWLEDGE_CSV_PATH", "data/knowledge.csv"),
		AllowedOrigins:           getEnvOrDefault("ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:3000"),
		AIRateLimit:              getEnvAsIntOrDefault("AI_RATE_LIMIT", 60),
		AIRateLimitWindowSeconds: getEnvAsIntOrDefault("AI_RATE_LIMIT_WINDOW_SECONDS", 3600),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	switch c.normalizedAIProvider() {
	case AIProviderGemini:
		if c.GeminiAPIKey == "" {
			return fmt.Errorf("GEMINI_API_KEY or GOOGLE_API_KEY is required")
		}
	case AIProviderMiniMax:
		if c.OpenRouterAPIKey == "" {
			return fmt.Errorf("OPENROUTER_API_KEY is required")
		}
		if c.MiniMaxAPIKey == "" {
			return fmt.Errorf("MINIMAX_API_KEY is required")
		}
		if c.OpenRouterBaseURL == "" {
			return fmt.Errorf("OPENROUTER_BASE_URL cannot be empty")
		}
		if c.OpenRouterOCRModel == "" {
			return fmt.Errorf("OPENROUTER_OCR_MODEL cannot be empty")
		}
		if c.MiniMaxAnthropicBaseURL == "" {
			return fmt.Errorf("MINIMAX_ANTHROPIC_BASE_URL cannot be empty")
		}
		if c.MiniMaxTextModel == "" {
			return fmt.Errorf("MINIMAX_TEXT_MODEL cannot be empty")
		}
		if c.MiniMaxTTSBaseURL == "" {
			return fmt.Errorf("MINIMAX_TTS_BASE_URL cannot be empty")
		}
		if c.MiniMaxTTSModel == "" {
			return fmt.Errorf("MINIMAX_TTS_MODEL cannot be empty")
		}
		if c.MiniMaxTTSVoiceID == "" {
			return fmt.Errorf("MINIMAX_TTS_VOICE_ID cannot be empty")
		}
	default:
		return fmt.Errorf("AI_PROVIDER must be one of: %s, %s", AIProviderMiniMax, AIProviderGemini)
	}
	if c.DBConnectionString == "" {
		return fmt.Errorf("DB_CONNECTION_STRING or PostgreSQL connection details are required")
	}
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if c.UploadDir == "" {
		return fmt.Errorf("UPLOAD_DIR cannot be empty")
	}
	if c.MaxUploadSize <= 0 {
		return fmt.Errorf("MAX_UPLOAD_SIZE must be positive")
	}
	if c.FrontendBaseURL == "" {
		return fmt.Errorf("FRONTEND_BASE_URL cannot be empty")
	}
	if c.TokenExpiryMinutes <= 0 {
		return fmt.Errorf("TOKEN_EXPIRY_MINUTES must be positive")
	}
	if c.DefaultPageSize <= 0 {
		return fmt.Errorf("DEFAULT_PAGE_SIZE must be positive")
	}
	if c.AIRateLimit < 0 {
		return fmt.Errorf("AI_RATE_LIMIT cannot be negative")
	}
	if c.AIRateLimitWindowSeconds < 0 {
		return fmt.Errorf("AI_RATE_LIMIT_WINDOW_SECONDS cannot be negative")
	}
	if c.AppEnv == "production" && c.AllowedOrigins == "*" {
		return fmt.Errorf("ALLOWED_ORIGINS cannot be wildcard in production")
	}
	if c.GoogleOAuthClientID == "" {
		return fmt.Errorf("GOOGLE_OAUTH_CLIENT_ID is required for Google login")
	}
	if c.GoogleOAuthClientSecret == "" {
		return fmt.Errorf("GOOGLE_OAUTH_CLIENT_SECRET is required for Google login")
	}
	return nil
}

func (c *Config) normalizedAIProvider() string {
	provider := strings.ToLower(strings.TrimSpace(c.AIProvider))
	if provider == "" {
		return AIProviderMiniMax
	}
	return provider
}

func getGeminiAPIKey() string {
	if value := os.Getenv("GEMINI_API_KEY"); value != "" {
		return value
	}
	return os.Getenv("GOOGLE_API_KEY")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt64OrDefault(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// GetAllowedOriginsList parses the AllowedOrigins config into a slice.
func (c *Config) GetAllowedOriginsList() []string {
	if c.AllowedOrigins == "" {
		return []string{"http://localhost:5173", "http://localhost:3000"}
	}
	if c.AllowedOrigins == "*" {
		return []string{"*"}
	}
	origins := strings.Split(c.AllowedOrigins, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}
	return origins
}

// IsOriginAllowed checks if a given origin is allowed based on config.
func (c *Config) IsOriginAllowed(origin string) bool {
	if c.AllowedOrigins == "*" {
		return true
	}
	allowed := c.GetAllowedOriginsList()
	for _, allowedOrigin := range allowed {
		if allowedOrigin == origin {
			return true
		}
	}
	return false
}

func loadEnvFile(filename string) {
	// Try multiple locations: current dir, parent dir (project root)
	paths := []string{
		filename,         // Current directory
		"../" + filename, // Parent directory (project root when running from backend/)
	}

	var file *os.File
	var err error
	for _, path := range paths {
		file, err = os.Open(path)
		if err == nil {
			break
		}
	}

	if file == nil {
		return // File not found in any location
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
			value = strings.Trim(value, `"`)
		} else if strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`) {
			value = strings.Trim(value, `'`)
		}

		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

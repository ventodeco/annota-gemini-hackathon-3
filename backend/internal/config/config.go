package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	GeminiAPIKey      string
	AppBaseURL        string
	Port              string
	DBConnectionString string
	UploadDir         string
	MaxUploadSize     int64
	SessionCookieName string
	SessionSecure     bool
}

func Load() (*Config, error) {
	loadEnvFile(".env.local")
	loadEnvFile(".env")

	geminiAPIKey := os.Getenv("GOOGLE_API_KEY")
	if geminiAPIKey == "" {
		geminiAPIKey = os.Getenv("GEMINI_API_KEY")
	}

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

	cfg := &Config{
		GeminiAPIKey:       geminiAPIKey,
		AppBaseURL:         getEnvOrDefault("APP_BASE_URL", "http://localhost:8080"),
		Port:               getEnvOrDefault("PORT", "8080"),
		DBConnectionString:  dbConnStr,
		UploadDir:          getEnvOrDefault("UPLOAD_DIR", "data/uploads"),
		MaxUploadSize:      getEnvAsInt64OrDefault("MAX_UPLOAD_SIZE", 10*1024*1024),
		SessionCookieName:  getEnvOrDefault("SESSION_COOKIE_NAME", "sid"),
		SessionSecure:      getEnvAsBoolOrDefault("SESSION_SECURE", false),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.GeminiAPIKey == "" {
		return fmt.Errorf("GEMINI_API_KEY or GOOGLE_API_KEY is required")
	}
	if c.DBConnectionString == "" {
		return fmt.Errorf("DB_CONNECTION_STRING or PostgreSQL connection details are required")
	}
	if c.UploadDir == "" {
		return fmt.Errorf("UPLOAD_DIR cannot be empty")
	}
	if c.MaxUploadSize <= 0 {
		return fmt.Errorf("MAX_UPLOAD_SIZE must be positive")
	}
	return nil
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

func getEnvAsBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func loadEnvFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		return
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

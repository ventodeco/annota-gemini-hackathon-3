package config

import (
	"strings"
	"testing"
)

func TestConfigValidateRejectsDefaultDBPasswordInProduction(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantErr bool
	}{
		{"url built-in default", "postgres://gemini_user:gemini_password@db:5432/gemini_db", true},
		{"postgresql url template default", "postgresql://postgres:password@db/gemini_db?sslmode=require", true},
		{"key value template default", "host=db port=5432 user=postgres password=password dbname=gemini_db sslmode=disable", true},
		{"key value quoted image default", "host=db user=postgres password='postgres' dbname=gemini_db", true},
		{"key value last password wins", "password=unique-fixture-value host=db password=postgres", true},
		{"url query password", "postgres://annota@db/annota?password=password", true},
		{"key value backslash escape", `host=db password=pass\word dbname=annota`, true},
		{"quoted backslash escape", `host=db password='pass\word' dbname=annota`, true},
		{"url query overrides userinfo", "postgres://annota:password@db/annota?password=unique-fixture-value", false},
		{"default inside another quoted value", "host=db password=unique-fixture-value application_name='some password=postgres tag'", false},
		{"url unique password", "postgres://annota:unique-fixture-value@db/annota", false},
		{"key value unique password", "host=db user=annota password=unique-fixture-value dbname=annota", false},
		{"quoted escaped password", `host=db password='pass\'word' dbname=annota`, false},
		{"password prefix only", "host=db password=password-but-longer dbname=annota", false},
		{"no password", "postgres://annota@db/annota", false},
		{"no password key value", "host=db user=annota dbname=annota", false},
	}

	t.Setenv("PGPASSWORD", "")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.AppEnv = "production"
			cfg.DBConnectionString = tt.dsn

			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && strings.Contains(err.Error(), "gemini_password") {
				t.Fatalf("error must not include the password: %v", err)
			}
		})
	}
}

func TestConfigValidateRejectsDefaultPGPASSWORDInProduction(t *testing.T) {
	t.Setenv("PGPASSWORD", "postgres")
	cfg := validConfig()
	cfg.AppEnv = "production"
	cfg.DBConnectionString = "host=db user=annota dbname=annota"

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected default PGPASSWORD to fail in production")
	}
}

func TestConfigValidateAllowsDefaultDBPasswordOutsideProduction(t *testing.T) {
	cfg := validConfig()
	cfg.AppEnv = "development"
	cfg.DBConnectionString = "host=localhost user=postgres password=password dbname=gemini_db sslmode=disable"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected local default password to be accepted outside production: %v", err)
	}
}

func TestLoadRejectsBuiltInDBPasswordDefaultInProduction(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("APP_ENV", "production")
	t.Setenv("ALLOWED_ORIGINS", "https://annota.example.com")
	t.Setenv("AI_PROVIDER", "minimax")
	t.Setenv("OPENROUTER_API_KEY", "openrouter-key")
	t.Setenv("MINIMAX_API_KEY", "minimax-key")
	t.Setenv("DB_CONNECTION_STRING", "")
	t.Setenv("POSTGRES_PASSWORD", "")
	setClerkTestEnv(t)

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "known default") {
		t.Fatalf("expected unset POSTGRES_PASSWORD to fail in production, got %v", err)
	}

	t.Setenv("POSTGRES_PASSWORD", "unique-fixture-value")
	if _, err := Load(); err != nil {
		t.Fatalf("expected unique POSTGRES_PASSWORD to load in production: %v", err)
	}
}

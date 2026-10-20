package config

import (
	"fmt"

	"github.com/lib/pq"
)

// knownDefaultDBPasswords are publicly visible defaults: the Load fallback,
// the backend/.env.example and docker-compose.yml value, and the common
// PostgreSQL image convention.
var knownDefaultDBPasswords = map[string]bool{
	"gemini_password": true,
	"password":        true,
	"postgres":        true,
}

// validateProductionDBPassword rejects a known default database password in
// production. It resolves the password with the same lib/pq parser the server
// uses, so DSN precedence, escaping, and PGPASSWORD match the real connection.
// A DSN the driver cannot parse is left for the driver to reject. The error
// never includes the password.
func (c *Config) validateProductionDBPassword() error {
	if c.AppEnv != "production" {
		return nil
	}
	pgCfg, err := pq.NewConfig(c.DBConnectionString)
	if err != nil {
		return nil
	}
	if knownDefaultDBPasswords[pgCfg.Password] {
		return fmt.Errorf("database password is a known default; set a unique POSTGRES_PASSWORD or DB_CONNECTION_STRING password in production")
	}
	return nil
}

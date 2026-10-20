package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// knownDefaultDBPasswords are publicly visible defaults: the Load fallback,
// the backend/.env.example and docker-compose.yml value, and the common
// PostgreSQL image convention.
var knownDefaultDBPasswords = map[string]bool{
	"gemini_password": true,
	"password":        true,
	"postgres":        true,
}

var keyValuePasswordPattern = regexp.MustCompile(`(?:^|\s)password\s*=\s*('(?:[^'\\]|\\.)*'|\S*)`)

// validateProductionDBPassword rejects a known default database password in
// production. The error never includes the password.
func (c *Config) validateProductionDBPassword() error {
	if c.AppEnv != "production" {
		return nil
	}
	password, ok := dbPasswordFromConnectionString(c.DBConnectionString)
	if ok && knownDefaultDBPasswords[password] {
		return fmt.Errorf("database password is a known default; set a unique POSTGRES_PASSWORD or DB_CONNECTION_STRING password in production")
	}
	return nil
}

// dbPasswordFromConnectionString returns the password in a PostgreSQL URL or
// key/value connection string, and false when none is present or parseable.
func dbPasswordFromConnectionString(dsn string) (string, bool) {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil || u.User == nil {
			return "", false
		}
		return u.User.Password()
	}

	matches := keyValuePasswordPattern.FindAllStringSubmatch(dsn, -1)
	if len(matches) == 0 {
		return "", false
	}
	value := matches[len(matches)-1][1]
	if len(value) >= 2 && strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
		value = strings.NewReplacer(`\'`, `'`, `\\`, `\`).Replace(value[1 : len(value)-1])
	}
	return value, true
}

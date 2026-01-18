package storage

import (
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func RunMigrations(db *sql.DB, migrationsDir string) error {
	if err := enableForeignKeys(db); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	migrations, err := readMigrationFiles(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migration files: %w", err)
	}

	if err := createMigrationsTable(db); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	for _, migration := range migrations {
		applied, err := isMigrationApplied(db, migration.Name)
		if err != nil {
			return fmt.Errorf("failed to check migration status: %w", err)
		}

		if applied {
			continue
		}

		if err := executeMigration(db, migration); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", migration.Name, err)
		}

		if err := recordMigration(db, migration.Name); err != nil {
			return fmt.Errorf("failed to record migration %s: %w", migration.Name, err)
		}
	}

	return nil
}

func enableForeignKeys(db *sql.DB) error {
	// PostgreSQL has foreign keys enabled by default, no action needed
	return nil
}

func createMigrationsTable(db *sql.DB) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL
		)
	`
	_, err := db.Exec(query)
	return err
}

func isMigrationApplied(db *sql.DB, name string) (bool, error) {
	var count int
	query := "SELECT COUNT(*) FROM schema_migrations WHERE name = $1"
	err := db.QueryRow(query, name).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func recordMigration(db *sql.DB, name string) error {
	query := "INSERT INTO schema_migrations (name, applied_at) VALUES ($1, NOW())"
	_, err := db.Exec(query, name)
	return err
}

type migrationFile struct {
	Name string
	Path string
	SQL  string
}

func readMigrationFiles(dir string) ([]migrationFile, error) {
	var migrations []migrationFile

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".sql") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", path, err)
		}

		migrations = append(migrations, migrationFile{
			Name: filepath.Base(path),
			Path: path,
			SQL:  string(content),
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Name < migrations[j].Name
	})

	return migrations, nil
}

func executeMigration(db *sql.DB, migration migrationFile) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(migration.SQL); err != nil {
		return fmt.Errorf("migration %s failed: %w", migration.Name, err)
	}

	return tx.Commit()
}

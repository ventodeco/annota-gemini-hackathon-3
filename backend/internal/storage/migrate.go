package storage

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// migrationLockKey identifies the database-wide advisory lock that serializes
// migration runs from concurrently starting servers.
const migrationLockKey int64 = 4_300_043

// migrationLockTimeout bounds how long a starter waits for another starter's
// migration before failing. A variable so tests can shorten it.
var migrationLockTimeout = 60 * time.Second

// RunMigrations applies pending PostgreSQL migrations in name order. Each
// migration's SQL and its ledger row commit in one transaction while holding a
// transaction-scoped advisory lock, so a failure leaves neither behind and
// concurrent starters apply each migration once. Migration files must
// therefore be transactional (for example, no CREATE INDEX CONCURRENTLY).
// Applied migrations absent from migrationsDir are left untouched so that a
// compatible older image can still start.
func RunMigrations(db *sql.DB, migrationsDir string) error {
	ctx := context.Background()

	migrations, err := readMigrationFiles(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migration files: %w", err)
	}
	if len(migrations) == 0 {
		return fmt.Errorf("no migration files found in %s", migrationsDir)
	}

	if err := withMigrationLock(ctx, db, createMigrationsTable); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	for _, migration := range migrations {
		if err := withMigrationLock(ctx, db, func(tx *sql.Tx) error {
			return applyMigration(ctx, tx, migration)
		}); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", migration.Name, err)
		}
	}

	return nil
}

// withMigrationLock runs fn in a transaction that first takes the migration
// advisory lock. The lock is released when the transaction ends.
func withMigrationLock(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	timeout := fmt.Sprintf("SET LOCAL lock_timeout = '%dms'", migrationLockTimeout.Milliseconds())
	if _, err := tx.ExecContext(ctx, timeout); err != nil {
		return fmt.Errorf("failed to bound migration lock wait: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", migrationLockKey); err != nil {
		return fmt.Errorf("failed to acquire migration lock within %s: %w", migrationLockTimeout, err)
	}
	if _, err := tx.ExecContext(ctx, "SET LOCAL lock_timeout TO DEFAULT"); err != nil {
		return fmt.Errorf("failed to restore lock timeout: %w", err)
	}

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func createMigrationsTable(tx *sql.Tx) error {
	_, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL
		)
	`)
	return err
}

func applyMigration(ctx context.Context, tx *sql.Tx, migration migrationFile) error {
	var applied bool
	if err := tx.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE name = $1)", migration.Name,
	).Scan(&applied); err != nil {
		return fmt.Errorf("failed to check migration status: %w", err)
	}
	if applied {
		return nil
	}

	if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
		return fmt.Errorf("migration SQL failed: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO schema_migrations (name, applied_at) VALUES ($1, NOW())", migration.Name,
	); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}
	return nil
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

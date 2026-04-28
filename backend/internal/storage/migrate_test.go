package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

func createTestMigrationsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	migration := `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email VARCHAR(255) NOT NULL,
			provider VARCHAR(50) NOT NULL,
			provider_id VARCHAR(255) NOT NULL,
			avatar_url TEXT,
			preferred_language VARCHAR(10) DEFAULT 'ID',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (provider, provider_id)
		);

		CREATE TABLE IF NOT EXISTS scans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
			image_url TEXT NOT NULL,
			full_ocr_text TEXT,
			detected_language VARCHAR(10),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS annotations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
			scan_id BIGINT REFERENCES scans(id) ON DELETE SET NULL,
			highlighted_text TEXT NOT NULL,
			context_text TEXT,
			nuance_data TEXT NOT NULL,
			is_bookmarked BOOLEAN DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_annotations_scan_id ON annotations(scan_id);
	`
	err := os.WriteFile(filepath.Join(dir, "001_schema.sql"), []byte(migration), 0644)
	if err != nil {
		t.Fatalf("failed to write migration file: %v", err)
	}
	return dir
}

func TestRunMigrations_CreatesTablesFromSQLFiles(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	migrationsDir := createTestMigrationsDir(t)

	// RunMigrations will error because recordMigration uses NOW() which SQLite
	// doesn't support. However, executeMigration commits its own transaction
	// before recordMigration runs, so the DDL should be persisted.
	err = RunMigrations(db, migrationsDir)
	if err == nil {
		t.Fatal("expected error from RunMigrations due to NOW() incompatibility with SQLite")
	}

	_, err = db.Exec("INSERT INTO users (email, provider, provider_id) VALUES ('a@b.com', 'google', 'g1')")
	if err != nil {
		t.Fatalf("users table should exist after migration: %v", err)
	}

	_, err = db.Exec("INSERT INTO scans (user_id, image_url) VALUES (1, '/img.jpg')")
	if err != nil {
		t.Fatalf("scans table should exist after migration: %v", err)
	}

	_, err = db.Exec("INSERT INTO annotations (user_id, highlighted_text, nuance_data) VALUES (1, 'text', '{}')")
	if err != nil {
		t.Fatalf("annotations table should exist after migration: %v", err)
	}

	var indexName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type = 'index' AND name = 'idx_annotations_scan_id'").Scan(&indexName)
	if err != nil {
		t.Fatalf("idx_annotations_scan_id should exist after migration: %v", err)
	}
}

func TestRunMigrations_IdempotentRerun(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	migrationsDir := createTestMigrationsDir(t)

	// First run will fail at recordMigration (NOW() unsupported in SQLite),
	// but executeMigration commits the DDL. Manually record the migration
	// with SQLite-compatible SQL so the second run can test the idempotency path.
	_ = RunMigrations(db, migrationsDir)

	_, err = db.Exec(
		"INSERT INTO schema_migrations (name, applied_at) VALUES ($1, CURRENT_TIMESTAMP)",
		"001_schema.sql",
	)
	if err != nil {
		t.Fatalf("failed to manually record migration: %v", err)
	}

	err = RunMigrations(db, migrationsDir)
	if err != nil {
		t.Errorf("second RunMigrations should succeed (migration already recorded), got: %v", err)
	}
}

func TestRunMigrations_EmptyDirectory(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	emptyDir := t.TempDir()

	err = RunMigrations(db, emptyDir)
	if err != nil {
		t.Fatalf("RunMigrations with empty dir should not error, got: %v", err)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("schema_migrations table should exist: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 migrations recorded, got %d", count)
	}
}

func TestRunProductionMigrations_Postgres(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set POSTGRES_TEST_DSN to run production PostgreSQL migration coverage")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open postgres: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	schema := fmt.Sprintf("migration_test_%d", time.Now().UnixNano())
	if _, err := db.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatalf("failed to create test schema: %v", err)
	}
	defer db.Exec("DROP SCHEMA " + schema + " CASCADE")
	if _, err := db.Exec("SET search_path TO " + schema); err != nil {
		t.Fatalf("failed to set search_path: %v", err)
	}

	if err := RunMigrations(db, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("RunMigrations against PostgreSQL failed: %v", err)
	}

	var tableName string
	if err := db.QueryRow("SELECT tablename FROM pg_tables WHERE schemaname = $1 AND tablename = 'users'", schema).Scan(&tableName); err != nil {
		t.Fatalf("users table should exist after production migrations: %v", err)
	}
}

func TestReadMigrationFiles_SortsAlphabetically(t *testing.T) {
	dir := t.TempDir()

	files := []string{"003_third.sql", "001_first.sql", "002_second.sql"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("SELECT 1;"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
	}

	migrations, err := readMigrationFiles(dir)
	if err != nil {
		t.Fatalf("readMigrationFiles returned error: %v", err)
	}

	if len(migrations) != 3 {
		t.Fatalf("expected 3 migrations, got %d", len(migrations))
	}
	if migrations[0].Name != "001_first.sql" {
		t.Errorf("first migration = %q, want %q", migrations[0].Name, "001_first.sql")
	}
	if migrations[1].Name != "002_second.sql" {
		t.Errorf("second migration = %q, want %q", migrations[1].Name, "002_second.sql")
	}
	if migrations[2].Name != "003_third.sql" {
		t.Errorf("third migration = %q, want %q", migrations[2].Name, "003_third.sql")
	}
}

func TestReadMigrationFiles_SkipsNonSQL(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "001_ok.sql"), []byte("SELECT 1;"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a migration"), 0644)
	os.WriteFile(filepath.Join(dir, "notes.md"), []byte("notes"), 0644)

	migrations, err := readMigrationFiles(dir)
	if err != nil {
		t.Fatalf("readMigrationFiles returned error: %v", err)
	}
	if len(migrations) != 1 {
		t.Errorf("expected 1 migration (only .sql), got %d", len(migrations))
	}
}

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

	migrationsDir := filepath.Join("..", "..", "migrations")
	legacyDir := t.TempDir()
	for _, name := range []string{
		"001_initial_database_schema.sql",
		"002_add_documents.sql",
		"003_prd_reader_memory.sql",
	} {
		content, err := os.ReadFile(filepath.Join(migrationsDir, name))
		if err != nil {
			t.Fatalf("failed to read legacy migration %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(legacyDir, name), content, 0600); err != nil {
			t.Fatalf("failed to copy legacy migration %s: %v", name, err)
		}
	}

	tests := []struct {
		name      string
		upgrade   bool
		dropIndex bool
	}{
		{name: "fresh"},
		{name: "existing_index", upgrade: true},
		{name: "missing_index", upgrade: true, dropIndex: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, schema := createPostgresMigrationSchema(t, dsn)
			var originalIndexID int64
			var annotations []migrationAnnotation
			if tt.upgrade {
				if err := RunMigrations(db, legacyDir); err != nil {
					t.Fatalf("failed to prepare legacy database: %v", err)
				}
				annotations = seedMigrationAnnotations(t, db)
				if tt.dropIndex {
					if _, err := db.Exec("DROP INDEX idx_annotations_scan_id"); err != nil {
						t.Fatalf("failed to remove legacy index: %v", err)
					}
					var count int
					if err := db.QueryRow("SELECT COUNT(*) FROM pg_indexes WHERE schemaname = $1 AND indexname = 'idx_annotations_scan_id'", schema).Scan(&count); err != nil {
						t.Fatalf("failed to inspect legacy indexes: %v", err)
					}
					if count != 0 {
						t.Fatal("legacy database must lack the scan index before upgrading")
					}
				} else {
					originalIndexID = annotationScanIndexID(t, db, schema)
				}
			}

			if err := RunMigrations(db, migrationsDir); err != nil {
				t.Fatalf("RunMigrations against PostgreSQL failed: %v", err)
			}
			var tableName string
			if err := db.QueryRow("SELECT tablename FROM pg_tables WHERE schemaname = $1 AND tablename = 'users'", schema).Scan(&tableName); err != nil {
				t.Fatalf("users table should exist after production migrations: %v", err)
			}
			indexID := annotationScanIndexID(t, db, schema)
			if originalIndexID != 0 && indexID != originalIndexID {
				t.Fatal("upgrade replaced the existing scan index")
			}
			if !tt.upgrade {
				annotations = seedMigrationAnnotations(t, db)
			}
			var migrationCount int
			if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&migrationCount); err != nil {
				t.Fatalf("failed to count applied migrations: %v", err)
			}

			for run := 0; run < 2; run++ {
				if run > 0 {
					if err := RunMigrations(db, migrationsDir); err != nil {
						t.Fatalf("rerunning production migrations failed: %v", err)
					}
					if got := annotationScanIndexID(t, db, schema); got != indexID {
						t.Fatal("rerunning migrations replaced the scan index")
					}
				}
				var total, indexMigrationCount int
				if err := db.QueryRow(`SELECT COUNT(*), COUNT(*) FILTER (WHERE name = '004_add_annotations_scan_id_index.sql') FROM schema_migrations`).Scan(&total, &indexMigrationCount); err != nil {
					t.Fatalf("failed to read migration history: %v", err)
				}
				if total != migrationCount || indexMigrationCount != 1 {
					t.Fatalf("migration run %d recorded %d migrations and %d index migrations; want %d and 1", run+1, total, indexMigrationCount, migrationCount)
				}
				for _, want := range annotations {
					var got migrationAnnotation
					if err := db.QueryRow("SELECT id, user_id, scan_id, highlighted_text FROM annotations WHERE id = $1", want.id).Scan(&got.id, &got.userID, &got.scanID, &got.text); err != nil {
						t.Fatalf("failed to read preserved annotation %d: %v", want.id, err)
					}
					if got != want {
						t.Fatalf("migration run %d changed annotation: got %+v, want %+v", run+1, got, want)
					}
				}
			}
		})
	}
}

func createPostgresMigrationSchema(t *testing.T, dsn string) (*sql.DB, string) {
	t.Helper()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open postgres: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close postgres: %v", err)
		}
	})
	db.SetMaxOpenConns(1)

	schema := fmt.Sprintf("migration_test_%d", time.Now().UnixNano())
	if _, err := db.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatalf("failed to create test schema: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Errorf("failed to drop test schema: %v", err)
		}
	})
	if _, err := db.Exec("SET search_path TO " + schema); err != nil {
		t.Fatalf("failed to set search_path: %v", err)
	}
	return db, schema
}

func annotationScanIndexID(t *testing.T, db *sql.DB, schema string) int64 {
	t.Helper()
	var id int64
	err := db.QueryRow(`
		SELECT i.oid
		FROM pg_index x
		JOIN pg_class i ON i.oid = x.indexrelid
		JOIN pg_class tbl ON tbl.oid = x.indrelid
		JOIN pg_namespace ns ON ns.oid = tbl.relnamespace AND ns.oid = i.relnamespace
		JOIN pg_attribute a ON a.attrelid = tbl.oid AND a.attnum = x.indkey[0]
		WHERE ns.nspname = $1 AND tbl.relname = 'annotations'
			AND i.relname = 'idx_annotations_scan_id' AND a.attname = 'scan_id'
			AND x.indisvalid AND NOT x.indisunique AND x.indnatts = 1
			AND x.indpred IS NULL AND x.indexprs IS NULL
	`, schema).Scan(&id)
	if err != nil {
		t.Fatalf("expected a valid non-unique annotations(scan_id) index in schema %s: %v", schema, err)
	}
	return id
}

type migrationAnnotation struct {
	id     int64
	userID int64
	scanID sql.NullInt64
	text   string
}

func seedMigrationAnnotations(t *testing.T, db *sql.DB) []migrationAnnotation {
	t.Helper()
	var userID, scanID int64
	if err := db.QueryRow("INSERT INTO users (email, provider, provider_id) VALUES ('migration@example.com', 'google', 'migration-user') RETURNING id").Scan(&userID); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	if err := db.QueryRow("INSERT INTO scans (user_id, image_url) VALUES ($1, '/migration.jpg') RETURNING id", userID).Scan(&scanID); err != nil {
		t.Fatalf("failed to seed scan: %v", err)
	}
	annotations := []migrationAnnotation{
		{userID: userID, scanID: sql.NullInt64{Int64: scanID, Valid: true}, text: "日本語"},
		{userID: userID, text: "unlinked annotation"},
	}
	for i := range annotations {
		annotation := &annotations[i]
		if err := db.QueryRow(`INSERT INTO annotations (user_id, scan_id, highlighted_text, nuance_data) VALUES ($1, $2, $3, '{}') RETURNING id`, annotation.userID, annotation.scanID, annotation.text).Scan(&annotation.id); err != nil {
			t.Fatalf("failed to seed annotation: %v", err)
		}
	}
	return annotations
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

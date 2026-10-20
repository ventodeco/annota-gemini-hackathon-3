package storage

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

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

func postgresTestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set POSTGRES_TEST_DSN to run PostgreSQL migration runner coverage")
	}
	return dsn
}

func writeMigrations(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatalf("failed to write migration %s: %v", name, err)
		}
	}
	return dir
}

func tableExists(t *testing.T, db *sql.DB, schema, table string) bool {
	t.Helper()
	var exists bool
	if err := db.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = $1 AND tablename = $2)", schema, table).Scan(&exists); err != nil {
		t.Fatalf("failed to inspect table %s: %v", table, err)
	}
	return exists
}

func recordedMigrations(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query("SELECT name FROM schema_migrations ORDER BY name")
	if err != nil {
		t.Fatalf("failed to read migration ledger: %v", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("failed to scan migration ledger: %v", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("failed to iterate migration ledger: %v", err)
	}
	return names
}

func TestRunMigrations_RollsBackFailedSQL_Postgres(t *testing.T) {
	db, schema := createPostgresMigrationSchema(t, postgresTestDSN(t))
	dir := writeMigrations(t, map[string]string{
		"001_ok.sql":   "CREATE TABLE first_table (id INT);",
		"002_fail.sql": "CREATE TABLE partial_table (id INT); SELECT missing_column FROM first_table;",
	})

	if err := RunMigrations(db, dir); err == nil {
		t.Fatal("expected failing migration SQL to return an error")
	}
	if !tableExists(t, db, schema, "first_table") {
		t.Fatal("successful earlier migration should remain applied")
	}
	if tableExists(t, db, schema, "partial_table") {
		t.Fatal("failed migration left partial DDL behind")
	}
	if got := strings.Join(recordedMigrations(t, db), ","); got != "001_ok.sql" {
		t.Fatalf("ledger = %q, want only 001_ok.sql", got)
	}
}

func TestRunMigrations_RollsBackSQLWhenLedgerWriteFails_Postgres(t *testing.T) {
	db, schema := createPostgresMigrationSchema(t, postgresTestDSN(t))
	// A ledger that rejects one name makes the bookkeeping insert fail after
	// the migration's SQL has run.
	if _, err := db.Exec(`CREATE TABLE schema_migrations (
		name VARCHAR(255) PRIMARY KEY CHECK (name <> '002_unrecordable.sql'),
		applied_at TIMESTAMP NOT NULL
	)`); err != nil {
		t.Fatalf("failed to create restricted ledger: %v", err)
	}
	dir := writeMigrations(t, map[string]string{
		"001_ok.sql":           "CREATE TABLE first_table (id INT);",
		"002_unrecordable.sql": "CREATE TABLE unrecorded_table (id INT);",
	})

	if err := RunMigrations(db, dir); err == nil {
		t.Fatal("expected ledger write failure to return an error")
	}
	if tableExists(t, db, schema, "unrecorded_table") {
		t.Fatal("migration SQL committed without its ledger row")
	}
	if got := strings.Join(recordedMigrations(t, db), ","); got != "001_ok.sql" {
		t.Fatalf("ledger = %q, want only 001_ok.sql", got)
	}
}

func TestRunMigrations_ConcurrentStarters_Postgres(t *testing.T) {
	dsn := postgresTestDSN(t)
	db, schema := createPostgresMigrationSchema(t, dsn)
	schemaDSN, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("POSTGRES_TEST_DSN must be a URL: %v", err)
	}
	query := schemaDSN.Query()
	query.Set("search_path", schema)
	schemaDSN.RawQuery = query.Encode()

	const starters = 4
	migrationsDir := filepath.Join("..", "..", "migrations")
	start := make(chan struct{})
	errs := make(chan error, starters)
	var wg sync.WaitGroup
	for i := 0; i < starters; i++ {
		starterDB, err := sql.Open("postgres", schemaDSN.String())
		if err != nil {
			t.Fatalf("failed to open starter connection: %v", err)
		}
		t.Cleanup(func() { starterDB.Close() })
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- RunMigrations(starterDB, migrationsDir)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent starter failed: %v", err)
		}
	}

	files, err := readMigrationFiles(migrationsDir)
	if err != nil {
		t.Fatalf("failed to read production migrations: %v", err)
	}
	var want []string
	for _, file := range files {
		want = append(want, file.Name)
	}
	if got := recordedMigrations(t, db); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ledger = %v, want each production migration once: %v", got, want)
	}
}

func TestRunMigrations_RejectsEmptyDirectory_Postgres(t *testing.T) {
	db, schema := createPostgresMigrationSchema(t, postgresTestDSN(t))

	if err := RunMigrations(db, t.TempDir()); err == nil {
		t.Fatal("expected an empty migration directory to fail startup")
	}
	if tableExists(t, db, schema, "schema_migrations") {
		t.Fatal("empty migration directory should not touch the database")
	}
}

func TestRunMigrations_BoundsLockWait_Postgres(t *testing.T) {
	dsn := postgresTestDSN(t)
	db, _ := createPostgresMigrationSchema(t, dsn)
	holder, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open lock holder: %v", err)
	}
	t.Cleanup(func() { holder.Close() })
	holderTx, err := holder.Begin()
	if err != nil {
		t.Fatalf("failed to begin lock holder: %v", err)
	}
	t.Cleanup(func() { holderTx.Rollback() })
	if _, err := holderTx.Exec("SELECT pg_advisory_xact_lock($1)", migrationLockKey); err != nil {
		t.Fatalf("failed to hold migration lock: %v", err)
	}

	previous := migrationLockTimeout
	migrationLockTimeout = 300 * time.Millisecond
	t.Cleanup(func() { migrationLockTimeout = previous })

	started := time.Now()
	err = RunMigrations(db, writeMigrations(t, map[string]string{"001_ok.sql": "CREATE TABLE first_table (id INT);"}))
	if err == nil || !strings.Contains(err.Error(), "migration lock") {
		t.Fatalf("expected a migration lock timeout error, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("lock wait took %s, want it bounded", elapsed)
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

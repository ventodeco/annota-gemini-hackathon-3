package storage

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestReadMigrationFilesSorted(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "002_second.sql"), []byte("SELECT 2;"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "001_first.sql"), []byte("SELECT 1;"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.txt"), []byte("skip"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	files, err := readMigrationFiles(dir)
	if err != nil {
		t.Fatalf("readMigrationFiles() error = %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("readMigrationFiles() len = %d, want 2", len(files))
	}
	if files[0].Name != "001_first.sql" || files[1].Name != "002_second.sql" {
		t.Fatalf("unexpected order: %q then %q", files[0].Name, files[1].Name)
	}
}

func TestRunMigrationsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "001_create.sql"), []byte("SELECT 1;"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	createTableQuery := regexp.QuoteMeta(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL
		)
	`)
	statusQuery := regexp.QuoteMeta("SELECT COUNT(*) FROM schema_migrations WHERE name = $1")
	recordQuery := regexp.QuoteMeta("INSERT INTO schema_migrations (name, applied_at) VALUES ($1, NOW())")

	mock.ExpectExec(createTableQuery).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(statusQuery).
		WithArgs("001_create.sql").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("SELECT 1;")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectExec(recordQuery).WithArgs("001_create.sql").WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(createTableQuery).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(statusQuery).
		WithArgs("001_create.sql").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	if err := RunMigrations(db, dir); err != nil {
		t.Fatalf("first RunMigrations() error = %v", err)
	}
	if err := RunMigrations(db, dir); err != nil {
		t.Fatalf("second RunMigrations() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("mock expectations: %v", err)
	}
}

package testutil

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/gemini-hackathon/app/internal/storage"
)

func CreateTestDB() (*sql.DB, string, error) {
	tmpDir, err := os.MkdirTemp("", "test_db_*")
	if err != nil {
		return nil, "", err
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, "", err
	}

	if err := storage.RunMigrations(db, "migrations"); err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		return nil, "", err
	}

	return db, tmpDir, nil
}

func CleanupTestDB(db *sql.DB, tmpDir string) {
	if db != nil {
		db.Close()
	}
	if tmpDir != "" {
		os.RemoveAll(tmpDir)
	}
}

func CreateInMemoryDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}

	if err := storage.RunMigrations(db, "migrations"); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

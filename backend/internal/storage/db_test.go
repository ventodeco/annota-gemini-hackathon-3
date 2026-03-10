package storage

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gemini-hackathon/app/internal/models"
)

func TestCreateUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	store := NewPostgresDB(db)
	now := time.Now()
	avatar := "https://example.com/avatar.png"
	user := &models.User{
		Email:             "user@example.com",
		Provider:          "google",
		ProviderID:        "provider-123",
		AvatarURL:         &avatar,
		PreferredLanguage: "ID",
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	query := regexp.QuoteMeta(`
		INSERT INTO users (email, provider, provider_id, avatar_url, preferred_language, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`)
	rows := sqlmock.NewRows([]string{"id"}).AddRow(int64(42))
	mock.ExpectQuery(query).
		WithArgs(user.Email, user.Provider, user.ProviderID, user.AvatarURL, user.PreferredLanguage, user.CreatedAt, user.UpdatedAt).
		WillReturnRows(rows)

	err = store.CreateUser(context.Background(), user)
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if user.ID != 42 {
		t.Fatalf("CreateUser() user.ID = %d, want 42", user.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("mock expectations: %v", err)
	}
}

func TestGetUserByEmailNoRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	store := NewPostgresDB(db)
	query := regexp.QuoteMeta(`
		SELECT id, email, provider, provider_id, avatar_url, preferred_language, created_at, updated_at
		FROM users
		WHERE email = $1
	`)
	mock.ExpectQuery(query).WithArgs("missing@example.com").WillReturnError(sql.ErrNoRows)

	user, err := store.GetUserByEmail(context.Background(), "missing@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail() error = %v, want nil", err)
	}
	if user != nil {
		t.Fatalf("GetUserByEmail() user = %#v, want nil", user)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("mock expectations: %v", err)
	}
}

func TestGetScansByUserID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	store := NewPostgresDB(db)
	now := time.Now()
	fullText := "ocr text"
	lang := "ja"
	query := regexp.QuoteMeta(`
		SELECT id, user_id, image_url, full_ocr_text, detected_language, created_at
		FROM scans
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`)
	rows := sqlmock.NewRows([]string{"id", "user_id", "image_url", "full_ocr_text", "detected_language", "created_at"}).
		AddRow(int64(10), int64(3), "/uploads/10.jpg", fullText, lang, now)
	mock.ExpectQuery(query).WithArgs(int64(3), 20, 0).WillReturnRows(rows)

	scans, err := store.GetScansByUserID(context.Background(), 3, 1, 20)
	if err != nil {
		t.Fatalf("GetScansByUserID() error = %v", err)
	}
	if len(scans) != 1 {
		t.Fatalf("GetScansByUserID() len = %d, want 1", len(scans))
	}
	if scans[0].ID != 10 {
		t.Fatalf("GetScansByUserID() id = %d, want 10", scans[0].ID)
	}
	if scans[0].FullOCRText == nil || *scans[0].FullOCRText != fullText {
		t.Fatalf("GetScansByUserID() FullOCRText = %v, want %q", scans[0].FullOCRText, fullText)
	}
	if scans[0].DetectedLanguage == nil || *scans[0].DetectedLanguage != lang {
		t.Fatalf("GetScansByUserID() DetectedLanguage = %v, want %q", scans[0].DetectedLanguage, lang)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("mock expectations: %v", err)
	}
}

func TestDeleteScanNoRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	store := NewPostgresDB(db)
	query := regexp.QuoteMeta(`DELETE FROM scans WHERE id = $1 AND user_id = $2`)
	mock.ExpectExec(query).
		WithArgs(int64(8), int64(4)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = store.DeleteScan(context.Background(), 8, 4)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("DeleteScan() error = %v, want %v", err, sql.ErrNoRows)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("mock expectations: %v", err)
	}
}

func TestGetAnnotationByIDInvalidNuanceJSON(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	store := NewPostgresDB(db)
	now := time.Now()
	query := regexp.QuoteMeta(`
		SELECT id, user_id, scan_id, highlighted_text, context_text, nuance_data, is_bookmarked, created_at
		FROM annotations
		WHERE id = $1
	`)
	rows := sqlmock.NewRows([]string{
		"id", "user_id", "scan_id", "highlighted_text", "context_text", "nuance_data", "is_bookmarked", "created_at",
	}).AddRow(int64(5), int64(9), int64(2), "言葉", "文脈", []byte("{invalid"), true, now)
	mock.ExpectQuery(query).WithArgs(int64(5)).WillReturnRows(rows)

	_, err = store.GetAnnotationByID(context.Background(), 5)
	if err == nil {
		t.Fatalf("GetAnnotationByID() error = nil, want json unmarshal error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("mock expectations: %v", err)
	}
}

package storage

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/gemini-hackathon/app/internal/models"
)

// setupTestDB creates an in-memory SQLite database with schema applied directly
// (bypassing RunMigrations to avoid NOW() incompatibility).
func setupTestDB(t *testing.T) DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	schema := `
		CREATE TABLE users (
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

		CREATE TABLE scans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
			image_url TEXT NOT NULL,
			full_ocr_text TEXT,
			detected_language VARCHAR(10),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE annotations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
			scan_id BIGINT REFERENCES scans(id) ON DELETE SET NULL,
			highlighted_text TEXT NOT NULL,
			context_text TEXT,
			nuance_data TEXT NOT NULL,
			is_bookmarked BOOLEAN DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX idx_scans_user_id ON scans(user_id);
		CREATE INDEX idx_annotations_user_id ON annotations(user_id);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	return NewPostgresDB(db)
}

func createTestUser(t *testing.T, store DB, suffix string) *models.User {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	avatarURL := "https://example.com/avatar_" + suffix + ".png"
	user := &models.User{
		Email:             "user_" + suffix + "@example.com",
		Provider:          "google",
		ProviderID:        "provider_" + suffix,
		AvatarURL:         &avatarURL,
		PreferredLanguage: "ID",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := store.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	return user
}

func createTestScan(t *testing.T, store DB, userID int64, imageURL string) *models.Scan {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	scan := &models.Scan{
		UserID:   userID,
		ImageURL: imageURL,
		CreatedAt: now,
	}
	id, err := store.CreateScan(context.Background(), scan)
	if err != nil {
		t.Fatalf("CreateScan failed: %v", err)
	}
	scan.ID = id
	return scan
}

// --- User Tests ---

func TestCreateUser(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	avatarURL := "https://example.com/avatar.png"
	user := &models.User{
		Email:             "test@example.com",
		Provider:          "google",
		ProviderID:        "goog_123",
		AvatarURL:         &avatarURL,
		PreferredLanguage: "EN",
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	err := store.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	if user.ID == 0 {
		t.Error("expected auto-generated ID to be non-zero")
	}
}

func TestGetUserByEmail_Found(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	created := createTestUser(t, store, "email_found")

	fetched, err := store.GetUserByEmail(ctx, created.Email)
	if err != nil {
		t.Fatalf("GetUserByEmail returned error: %v", err)
	}
	if fetched == nil {
		t.Fatal("expected user, got nil")
	}
	if fetched.ID != created.ID {
		t.Errorf("ID mismatch: got %d, want %d", fetched.ID, created.ID)
	}
	if fetched.Email != created.Email {
		t.Errorf("Email mismatch: got %q, want %q", fetched.Email, created.Email)
	}
	if fetched.Provider != created.Provider {
		t.Errorf("Provider mismatch: got %q, want %q", fetched.Provider, created.Provider)
	}
}

func TestGetUserByEmail_NotFound(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	fetched, err := store.GetUserByEmail(ctx, "nonexistent@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail returned error: %v", err)
	}
	if fetched != nil {
		t.Errorf("expected nil for non-existent user, got %+v", fetched)
	}
}

func TestGetUserByProvider(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	created := createTestUser(t, store, "prov")

	fetched, err := store.GetUserByProvider(ctx, created.Provider, created.ProviderID)
	if err != nil {
		t.Fatalf("GetUserByProvider returned error: %v", err)
	}
	if fetched == nil {
		t.Fatal("expected user, got nil")
	}
	if fetched.ID != created.ID {
		t.Errorf("ID mismatch: got %d, want %d", fetched.ID, created.ID)
	}

	// Not found case
	fetched, err = store.GetUserByProvider(ctx, "github", "unknown_id")
	if err != nil {
		t.Fatalf("GetUserByProvider returned error: %v", err)
	}
	if fetched != nil {
		t.Errorf("expected nil for non-existent provider, got %+v", fetched)
	}
}

func TestGetUserByID(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	created := createTestUser(t, store, "byid")

	fetched, err := store.GetUserByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetUserByID returned error: %v", err)
	}
	if fetched == nil {
		t.Fatal("expected user, got nil")
	}
	if fetched.Email != created.Email {
		t.Errorf("Email mismatch: got %q, want %q", fetched.Email, created.Email)
	}

	// Not found
	fetched, err = store.GetUserByID(ctx, 99999)
	if err != nil {
		t.Fatalf("GetUserByID returned error: %v", err)
	}
	if fetched != nil {
		t.Errorf("expected nil for non-existent ID, got %+v", fetched)
	}
}

func TestUpdateUserLanguage(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	created := createTestUser(t, store, "lang")

	err := store.UpdateUserLanguage(ctx, created.ID, "JA")
	if err != nil {
		t.Fatalf("UpdateUserLanguage returned error: %v", err)
	}

	fetched, err := store.GetUserByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetUserByID returned error: %v", err)
	}
	if fetched.PreferredLanguage != "JA" {
		t.Errorf("PreferredLanguage = %q, want %q", fetched.PreferredLanguage, "JA")
	}
}

func TestCreateUser_AvatarNil(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	user := &models.User{
		Email:             "noavatar@example.com",
		Provider:          "github",
		ProviderID:        "gh_123",
		AvatarURL:         nil,
		PreferredLanguage: "EN",
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := store.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser with nil avatar returned error: %v", err)
	}

	fetched, err := store.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID returned error: %v", err)
	}
	if fetched.AvatarURL != nil {
		t.Errorf("expected nil AvatarURL, got %v", *fetched.AvatarURL)
	}
}

// --- Scan Tests ---

func TestCreateScan(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	user := createTestUser(t, store, "scan_create")

	now := time.Now().UTC().Truncate(time.Second)
	scan := &models.Scan{
		UserID:   user.ID,
		ImageURL: "/uploads/test.jpg",
		CreatedAt: now,
	}

	id, err := store.CreateScan(ctx, scan)
	if err != nil {
		t.Fatalf("CreateScan returned error: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero scan ID")
	}
}

func TestGetScanByID(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	user := createTestUser(t, store, "scan_get")
	scan := createTestScan(t, store, user.ID, "/uploads/img1.jpg")

	fetched, err := store.GetScanByID(ctx, scan.ID)
	if err != nil {
		t.Fatalf("GetScanByID returned error: %v", err)
	}
	if fetched.UserID != user.ID {
		t.Errorf("UserID = %d, want %d", fetched.UserID, user.ID)
	}
	if fetched.ImageURL != "/uploads/img1.jpg" {
		t.Errorf("ImageURL = %q, want %q", fetched.ImageURL, "/uploads/img1.jpg")
	}
}

func TestGetScanByID_NotFound(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, err := store.GetScanByID(ctx, 99999)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got: %v", err)
	}
}

func TestGetScansByUserID_PaginationAndOrdering(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	user := createTestUser(t, store, "scan_list")

	// Create 5 scans with different timestamps
	for i := 0; i < 5; i++ {
		ts := time.Date(2025, 1, 1, i, 0, 0, 0, time.UTC)
		scan := &models.Scan{
			UserID:    user.ID,
			ImageURL:  "/uploads/img.jpg",
			CreatedAt: ts,
		}
		if _, err := store.CreateScan(ctx, scan); err != nil {
			t.Fatalf("CreateScan %d failed: %v", i, err)
		}
	}

	// Page 1, size 3 - should get 3 most recent
	scans, err := store.GetScansByUserID(ctx, user.ID, 1, 3)
	if err != nil {
		t.Fatalf("GetScansByUserID page 1 returned error: %v", err)
	}
	if len(scans) != 3 {
		t.Fatalf("expected 3 scans on page 1, got %d", len(scans))
	}
	// Verify DESC ordering
	if !scans[0].CreatedAt.After(scans[1].CreatedAt) {
		t.Error("scans should be ordered by created_at DESC")
	}

	// Page 2, size 3 - should get remaining 2
	scans, err = store.GetScansByUserID(ctx, user.ID, 2, 3)
	if err != nil {
		t.Fatalf("GetScansByUserID page 2 returned error: %v", err)
	}
	if len(scans) != 2 {
		t.Fatalf("expected 2 scans on page 2, got %d", len(scans))
	}

	// Empty result for different user
	scans, err = store.GetScansByUserID(ctx, 99999, 1, 10)
	if err != nil {
		t.Fatalf("GetScansByUserID for other user returned error: %v", err)
	}
	if len(scans) != 0 {
		t.Errorf("expected 0 scans for non-existent user, got %d", len(scans))
	}
}

func TestUpdateScanImageURL(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	user := createTestUser(t, store, "scan_imgurl")
	scan := createTestScan(t, store, user.ID, "/uploads/old.jpg")

	err := store.UpdateScanImageURL(ctx, scan.ID, "/uploads/new.jpg")
	if err != nil {
		t.Fatalf("UpdateScanImageURL returned error: %v", err)
	}

	fetched, err := store.GetScanByID(ctx, scan.ID)
	if err != nil {
		t.Fatalf("GetScanByID returned error: %v", err)
	}
	if fetched.ImageURL != "/uploads/new.jpg" {
		t.Errorf("ImageURL = %q, want %q", fetched.ImageURL, "/uploads/new.jpg")
	}
}

func TestUpdateScanOCR(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	user := createTestUser(t, store, "scan_ocr")
	scan := createTestScan(t, store, user.ID, "/uploads/ocr.jpg")

	err := store.UpdateScanOCR(ctx, scan.ID, "recognized text here", "ja")
	if err != nil {
		t.Fatalf("UpdateScanOCR returned error: %v", err)
	}

	fetched, err := store.GetScanByID(ctx, scan.ID)
	if err != nil {
		t.Fatalf("GetScanByID returned error: %v", err)
	}
	if fetched.FullOCRText == nil || *fetched.FullOCRText != "recognized text here" {
		t.Errorf("FullOCRText = %v, want %q", fetched.FullOCRText, "recognized text here")
	}
	if fetched.DetectedLanguage == nil || *fetched.DetectedLanguage != "ja" {
		t.Errorf("DetectedLanguage = %v, want %q", fetched.DetectedLanguage, "ja")
	}
}

func TestDeleteScan(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	user := createTestUser(t, store, "scan_del")
	scan := createTestScan(t, store, user.ID, "/uploads/del.jpg")

	// Delete with correct owner
	err := store.DeleteScan(ctx, scan.ID, user.ID)
	if err != nil {
		t.Fatalf("DeleteScan returned error: %v", err)
	}

	// Verify deleted
	_, err = store.GetScanByID(ctx, scan.ID)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows after delete, got: %v", err)
	}
}

func TestDeleteScan_WrongUser(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	user := createTestUser(t, store, "scan_del_wrong")
	scan := createTestScan(t, store, user.ID, "/uploads/wrong.jpg")

	// Delete with wrong user ID
	err := store.DeleteScan(ctx, scan.ID, 99999)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows for wrong user, got: %v", err)
	}

	// Verify scan still exists
	fetched, err := store.GetScanByID(ctx, scan.ID)
	if err != nil {
		t.Fatalf("scan should still exist, GetScanByID returned error: %v", err)
	}
	if fetched == nil {
		t.Error("scan should still exist after failed delete")
	}
}

// --- Annotation Tests ---

func TestCreateAnnotation_WithNuanceDataRoundTrip(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	user := createTestUser(t, store, "ann_create")
	scan := createTestScan(t, store, user.ID, "/uploads/ann.jpg")

	now := time.Now().UTC().Truncate(time.Second)
	scanID := scan.ID
	contextText := "full sentence context"
	annotation := &models.Annotation{
		UserID:          user.ID,
		ScanID:          &scanID,
		HighlightedText: "highlighted word",
		ContextText:     &contextText,
		NuanceData: models.NuanceData{
			Meaning:            "the meaning",
			UsageExample:       "example usage",
			UsageTiming:        "formal",
			WordBreakdown:      "word + breakdown",
			AlternativeMeaning: "alt meaning",
		},
		IsBookmarked: true,
		CreatedAt:    now,
	}

	id, err := store.CreateAnnotation(ctx, annotation)
	if err != nil {
		t.Fatalf("CreateAnnotation returned error: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero annotation ID")
	}

	// Round-trip: fetch and verify NuanceData
	fetched, err := store.GetAnnotationByID(ctx, id)
	if err != nil {
		t.Fatalf("GetAnnotationByID returned error: %v", err)
	}

	if fetched.HighlightedText != "highlighted word" {
		t.Errorf("HighlightedText = %q, want %q", fetched.HighlightedText, "highlighted word")
	}
	if fetched.ContextText == nil || *fetched.ContextText != "full sentence context" {
		t.Errorf("ContextText = %v, want %q", fetched.ContextText, "full sentence context")
	}
	if fetched.ScanID == nil || *fetched.ScanID != scanID {
		t.Errorf("ScanID = %v, want %d", fetched.ScanID, scanID)
	}
	if fetched.NuanceData.Meaning != "the meaning" {
		t.Errorf("NuanceData.Meaning = %q, want %q", fetched.NuanceData.Meaning, "the meaning")
	}
	if fetched.NuanceData.UsageExample != "example usage" {
		t.Errorf("NuanceData.UsageExample = %q, want %q", fetched.NuanceData.UsageExample, "example usage")
	}
	if fetched.NuanceData.UsageTiming != "formal" {
		t.Errorf("NuanceData.UsageTiming = %q, want %q", fetched.NuanceData.UsageTiming, "formal")
	}
	if fetched.NuanceData.WordBreakdown != "word + breakdown" {
		t.Errorf("NuanceData.WordBreakdown = %q, want %q", fetched.NuanceData.WordBreakdown, "word + breakdown")
	}
	if fetched.NuanceData.AlternativeMeaning != "alt meaning" {
		t.Errorf("NuanceData.AlternativeMeaning = %q, want %q", fetched.NuanceData.AlternativeMeaning, "alt meaning")
	}
	if !fetched.IsBookmarked {
		t.Error("expected IsBookmarked to be true")
	}
}

func TestGetAnnotationByID_NotFound(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, err := store.GetAnnotationByID(ctx, 99999)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got: %v", err)
	}
}

func TestGetAnnotationsByUserID_Pagination(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	user := createTestUser(t, store, "ann_list")
	scan := createTestScan(t, store, user.ID, "/uploads/annlist.jpg")

	scanID := scan.ID

	// Create 5 annotations with different timestamps
	for i := 0; i < 5; i++ {
		ts := time.Date(2025, 1, 1, i, 0, 0, 0, time.UTC)
		ann := &models.Annotation{
			UserID:          user.ID,
			ScanID:          &scanID,
			HighlightedText: "word",
			NuanceData:      models.NuanceData{Meaning: "meaning"},
			IsBookmarked:    false,
			CreatedAt:       ts,
		}
		if _, err := store.CreateAnnotation(ctx, ann); err != nil {
			t.Fatalf("CreateAnnotation %d failed: %v", i, err)
		}
	}

	// Page 1, size 3
	anns, err := store.GetAnnotationsByUserID(ctx, user.ID, 1, 3)
	if err != nil {
		t.Fatalf("GetAnnotationsByUserID page 1 returned error: %v", err)
	}
	if len(anns) != 3 {
		t.Fatalf("expected 3 annotations on page 1, got %d", len(anns))
	}
	// Verify DESC ordering
	if !anns[0].CreatedAt.After(anns[1].CreatedAt) {
		t.Error("annotations should be ordered by created_at DESC")
	}

	// Page 2, size 3
	anns, err = store.GetAnnotationsByUserID(ctx, user.ID, 2, 3)
	if err != nil {
		t.Fatalf("GetAnnotationsByUserID page 2 returned error: %v", err)
	}
	if len(anns) != 2 {
		t.Fatalf("expected 2 annotations on page 2, got %d", len(anns))
	}
}

func TestGetAnnotationsByUserIDAndScanID(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	user := createTestUser(t, store, "ann_scan_filter")
	scan1 := createTestScan(t, store, user.ID, "/uploads/s1.jpg")
	scan2 := createTestScan(t, store, user.ID, "/uploads/s2.jpg")

	scan1ID := scan1.ID
	scan2ID := scan2.ID

	// Create 3 annotations for scan1, 2 for scan2
	for i := 0; i < 3; i++ {
		ann := &models.Annotation{
			UserID:          user.ID,
			ScanID:          &scan1ID,
			HighlightedText: "word",
			NuanceData:      models.NuanceData{Meaning: "m"},
			CreatedAt:       time.Now().UTC(),
		}
		if _, err := store.CreateAnnotation(ctx, ann); err != nil {
			t.Fatalf("CreateAnnotation failed: %v", err)
		}
	}
	for i := 0; i < 2; i++ {
		ann := &models.Annotation{
			UserID:          user.ID,
			ScanID:          &scan2ID,
			HighlightedText: "other",
			NuanceData:      models.NuanceData{Meaning: "m"},
			CreatedAt:       time.Now().UTC(),
		}
		if _, err := store.CreateAnnotation(ctx, ann); err != nil {
			t.Fatalf("CreateAnnotation failed: %v", err)
		}
	}

	anns, err := store.GetAnnotationsByUserIDAndScanID(ctx, user.ID, scan1ID, 1, 10)
	if err != nil {
		t.Fatalf("GetAnnotationsByUserIDAndScanID returned error: %v", err)
	}
	if len(anns) != 3 {
		t.Errorf("expected 3 annotations for scan1, got %d", len(anns))
	}

	anns, err = store.GetAnnotationsByUserIDAndScanID(ctx, user.ID, scan2ID, 1, 10)
	if err != nil {
		t.Fatalf("GetAnnotationsByUserIDAndScanID returned error: %v", err)
	}
	if len(anns) != 2 {
		t.Errorf("expected 2 annotations for scan2, got %d", len(anns))
	}
}

func TestDeleteAnnotation(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	user := createTestUser(t, store, "ann_del")
	scan := createTestScan(t, store, user.ID, "/uploads/anndel.jpg")
	scanID := scan.ID

	ann := &models.Annotation{
		UserID:          user.ID,
		ScanID:          &scanID,
		HighlightedText: "delete me",
		NuanceData:      models.NuanceData{Meaning: "m"},
		CreatedAt:       time.Now().UTC(),
	}
	id, err := store.CreateAnnotation(ctx, ann)
	if err != nil {
		t.Fatalf("CreateAnnotation failed: %v", err)
	}

	// Delete with correct owner
	err = store.DeleteAnnotation(ctx, id, user.ID)
	if err != nil {
		t.Fatalf("DeleteAnnotation returned error: %v", err)
	}

	// Verify deleted
	_, err = store.GetAnnotationByID(ctx, id)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows after delete, got: %v", err)
	}
}

func TestDeleteAnnotation_WrongUser(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	user := createTestUser(t, store, "ann_del_wrong")
	scan := createTestScan(t, store, user.ID, "/uploads/annwrong.jpg")
	scanID := scan.ID

	ann := &models.Annotation{
		UserID:          user.ID,
		ScanID:          &scanID,
		HighlightedText: "keep me",
		NuanceData:      models.NuanceData{Meaning: "m"},
		CreatedAt:       time.Now().UTC(),
	}
	id, err := store.CreateAnnotation(ctx, ann)
	if err != nil {
		t.Fatalf("CreateAnnotation failed: %v", err)
	}

	// Delete with wrong user
	err = store.DeleteAnnotation(ctx, id, 99999)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows for wrong user, got: %v", err)
	}

	// Verify still exists
	fetched, err := store.GetAnnotationByID(ctx, id)
	if err != nil {
		t.Fatalf("annotation should still exist, got error: %v", err)
	}
	if fetched == nil {
		t.Error("annotation should still exist after failed delete")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

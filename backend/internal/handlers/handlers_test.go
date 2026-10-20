package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gemini-hackathon/app/internal/auth"
	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/handlers"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/storage"
	"github.com/gemini-hackathon/app/internal/testutil"
)

var errDeleteFile = errors.New("delete file failed")

type failingDeleteFileStorage struct {
	mockFileStorage
}

func (m *failingDeleteFileStorage) DeleteImage(path string) error {
	return errDeleteFile
}

func (m *failingDeleteFileStorage) DeletePDF(path string) error {
	return errDeleteFile
}

type fakeIdentityDirectory struct {
	deleteErrs []error
	deleted    []string
}

func (f *fakeIdentityDirectory) GetIdentity(ctx context.Context, clerkUserID string) (*auth.ClerkIdentity, error) {
	return nil, auth.ErrIdentityNotFound
}

func (f *fakeIdentityDirectory) DeleteUser(ctx context.Context, clerkUserID string) error {
	if len(f.deleteErrs) > 0 {
		err := f.deleteErrs[0]
		f.deleteErrs = f.deleteErrs[1:]
		if err != nil {
			return err
		}
	}
	f.deleted = append(f.deleted, clerkUserID)
	return nil
}

func TestUserHandlers(t *testing.T) {
	mockDB := testutil.NewMockDB()
	userHandlers := handlers.NewUserHandlers(mockDB)

	user := &models.User{
		ID:                1,
		Email:             "test@example.com",
		Provider:          "google",
		ProviderID:        "google-123",
		PreferredLanguage: "ID",
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	mockDB.CreateUser(context.Background(), user)

	t.Run("GetLanguagesAPI", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/users/me/languages", nil)
		rec := httptest.NewRecorder()
		userHandlers.GetLanguagesAPI(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "ID") || !strings.Contains(body, "JP") || !strings.Contains(body, "EN") {
			t.Error("Response should contain ID, JP, EN languages")
		}
	})

	t.Run("GetUserProfileAPI_Authorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/users/me", nil)
		ctx := middleware.WithUserID(req.Context(), 1)
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		userHandlers.GetUserProfileAPI(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "ID") {
			t.Error("Response should contain preferred language")
		}
	})

	t.Run("GetUserProfileAPI_Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/users/me", nil)
		rec := httptest.NewRecorder()
		userHandlers.GetUserProfileAPI(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
	})

	t.Run("UpdateUserPreferencesAPI", func(t *testing.T) {
		body := `{"preferredLanguage": "JP"}`
		req := httptest.NewRequest("PATCH", "/v1/users/me", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := middleware.WithUserID(req.Context(), 1)
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		userHandlers.UpdateUserPreferencesAPI(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		body = rec.Body.String()
		if !strings.Contains(body, "JP") {
			t.Error("Response should contain updated language")
		}
	})

	t.Run("UpdateUserPreferencesAPI_InvalidLanguage", func(t *testing.T) {
		body := `{"preferredLanguage": "INVALID"}`
		req := httptest.NewRequest("PATCH", "/v1/users/me", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := middleware.WithUserID(req.Context(), 1)
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		userHandlers.UpdateUserPreferencesAPI(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})
}

func TestDeleteAccountRemovesOwnedRowsAndFiles(t *testing.T) {
	mockDB := testutil.NewMockDB()
	cfg := &config.Config{UploadDir: "data/uploads"}
	userHandlers := handlers.NewUserHandlersWithStorage(mockDB, &mockFileStorage{}, cfg, &fakeIdentityDirectory{})

	user := &models.User{
		Email:             "delete@example.com",
		Provider:          "google",
		ProviderID:        "delete-123",
		PreferredLanguage: "EN",
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	if err := mockDB.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/v1/users/me", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), user.ID))
	rec := httptest.NewRecorder()

	userHandlers.UsersMeAPI(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", rec.Code, rec.Body.String())
	}

	deleted, err := mockDB.GetUserByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetUserByID returned error: %v", err)
	}
	if deleted != nil {
		t.Fatalf("expected user to be deleted")
	}
}

func TestDeleteAccountFailsWhenOwnedFileDeletionFails(t *testing.T) {
	mockDB := testutil.NewMockDB()
	cfg := &config.Config{UploadDir: "data/uploads"}
	userHandlers := handlers.NewUserHandlersWithStorage(mockDB, &failingDeleteFileStorage{}, cfg, &fakeIdentityDirectory{})

	user := &models.User{
		Email:             "delete-fail@example.com",
		Provider:          "google",
		ProviderID:        "delete-fail-123",
		PreferredLanguage: "EN",
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	if err := mockDB.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	if _, err := mockDB.CreateScan(context.Background(), &models.Scan{
		UserID:   user.ID,
		ImageURL: "/v1/scans/1/image",
	}); err != nil {
		t.Fatalf("CreateScan returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/v1/users/me", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), user.ID))
	rec := httptest.NewRecorder()

	userHandlers.UsersMeAPI(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
	}

	existing, err := mockDB.GetUserByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetUserByID returned error: %v", err)
	}
	if existing == nil {
		t.Fatalf("expected user to remain when file purge fails")
	}
}

func TestScanHandlers(t *testing.T) {
	mockDB := testutil.NewMockDB()
	cfg := &config.Config{DefaultPageSize: 20, MaxUploadSize: 10 * 1024 * 1024}
	scanHandlers := handlers.NewScanHandlers(mockDB, nil, nil, cfg, newTestMeter(t))

	t.Run("GetScansAPI_Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/scans", nil)
		rec := httptest.NewRecorder()
		scanHandlers.GetScansAPI(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
	})

	t.Run("GetScansAPI_Authorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/scans?page=1&size=10", nil)
		ctx := middleware.WithUserID(req.Context(), 1)
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		scanHandlers.GetScansAPI(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "data") {
			t.Error("Response should contain 'data' field")
		}
		if !strings.Contains(body, "meta") {
			t.Error("Response should contain 'meta' field")
		}
	})

	t.Run("GetScansAPI_WithPagination", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/scans?page=2&size=5", nil)
		ctx := middleware.WithUserID(req.Context(), 1)
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		scanHandlers.GetScansAPI(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})

	t.Run("GetScanAPI_Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/scans/1", nil)
		rec := httptest.NewRecorder()
		scanHandlers.GetScanAPI(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
	})

	t.Run("GetScanAPI_InvalidID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/scans/abc", nil)
		ctx := middleware.WithUserID(req.Context(), 1)
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		scanHandlers.GetScanAPI(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})

	t.Run("GetScanAPI_NotFound", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/scans/999", nil)
		ctx := middleware.WithUserID(req.Context(), 1)
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		scanHandlers.GetScanAPI(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", rec.Code)
		}
	})
}

func TestAnnotationHandlers(t *testing.T) {
	mockDB := testutil.NewMockDB()
	cfg := &config.Config{DefaultPageSize: 20}
	annotationHandlers := handlers.NewAnnotationHandlers(mockDB, cfg)

	t.Run("GetAnnotationsAPI_Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/annotations", nil)
		rec := httptest.NewRecorder()
		annotationHandlers.GetAnnotationsAPI(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
	})

	t.Run("GetAnnotationsAPI_Authorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/annotations?page=1&size=10", nil)
		ctx := middleware.WithUserID(req.Context(), 1)
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		annotationHandlers.GetAnnotationsAPI(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "data") {
			t.Error("Response should contain 'data' field")
		}
	})

	t.Run("GetAnnotationsAPI_WithValidScanID", func(t *testing.T) {
		scanIDOne := int64(1)
		scanIDTwo := int64(2)
		mockDB.CreateAnnotation(context.Background(), &models.Annotation{
			UserID:          1,
			ScanID:          &scanIDOne,
			HighlightedText: "first",
			NuanceData:      models.NuanceData{Meaning: "one"},
			IsBookmarked:    true,
			CreatedAt:       time.Now(),
		})
		mockDB.CreateAnnotation(context.Background(), &models.Annotation{
			UserID:          1,
			ScanID:          &scanIDTwo,
			HighlightedText: "second",
			NuanceData:      models.NuanceData{Meaning: "two"},
			IsBookmarked:    true,
			CreatedAt:       time.Now(),
		})

		req := httptest.NewRequest("GET", "/v1/annotations?page=1&size=10&scanId=2", nil)
		ctx := middleware.WithUserID(req.Context(), 1)
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		annotationHandlers.GetAnnotationsAPI(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", rec.Code)
		}

		var response struct {
			Data []struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse JSON response: %v", err)
		}

		if len(response.Data) != 1 {
			t.Fatalf("Expected exactly 1 filtered annotation, got %d", len(response.Data))
		}
		if response.Data[0].ID != 2 {
			t.Fatalf("Expected annotation ID 2, got %d", response.Data[0].ID)
		}
	})

	t.Run("GetAnnotationsAPI_InvalidScanID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/annotations?page=1&size=10&scanId=abc", nil)
		ctx := middleware.WithUserID(req.Context(), 1)
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		annotationHandlers.GetAnnotationsAPI(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})

	t.Run("GetAnnotationsAPI_ScanIDMustBePositive", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/annotations?page=1&size=10&scanId=0", nil)
		ctx := middleware.WithUserID(req.Context(), 1)
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		annotationHandlers.GetAnnotationsAPI(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})

	t.Run("CreateAnnotationAPI_MissingHighlightedText", func(t *testing.T) {
		body := `{"scanId": 1, "contextText": "test context"}`
		req := httptest.NewRequest("POST", "/v1/annotations", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := middleware.WithUserID(req.Context(), 1)
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		annotationHandlers.CreateAnnotationAPI(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})

	t.Run("CreateAnnotationAPI_Success", func(t *testing.T) {
		mockDB.CreateScan(context.Background(), &models.Scan{
			UserID:     1,
			ImageURL:   "/uploads/test.jpg",
			SourceType: "image",
			Status:     "ready",
			CreatedAt:  time.Now(),
		})
		body := `{"scanId": 1, "highlightedText": "test text", "contextText": "test context", "nuanceData": {"meaning": "test meaning", "usageExample": "test example", "usageTiming": "test timing", "wordBreakdown": "test breakdown", "alternativeMeaning": "test alt"}}`
		req := httptest.NewRequest("POST", "/v1/annotations", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := middleware.WithUserID(req.Context(), 1)
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		annotationHandlers.CreateAnnotationAPI(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", rec.Code)
		}

		responseBody := rec.Body.String()
		if !strings.Contains(responseBody, "annotationId") {
			t.Error("Response should contain 'annotationId'")
		}
		if !strings.Contains(responseBody, "saved") {
			t.Error("Response should contain 'status': 'saved'")
		}
	})
}

func TestDeleteAccountTombstonesAndRetriesClerkDeletion(t *testing.T) {
	ctx := context.Background()
	mockDB := testutil.NewMockDB()
	directory := &fakeIdentityDirectory{deleteErrs: []error{auth.ErrIdentityUnavailable}}
	userHandlers := handlers.NewUserHandlersWithStorage(mockDB, &mockFileStorage{}, &config.Config{UploadDir: "data/uploads"}, directory)
	resolver := auth.NewIdentityResolver(mockDB, directory)

	user, err := mockDB.ProvisionClerkUser(ctx, storage.ClerkProvisionParams{ClerkUserID: "user_del", Email: "del@example.com"})
	if err != nil {
		t.Fatalf("ProvisionClerkUser: %v", err)
	}

	deleteRequest := func(userID int64) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodDelete, "/v1/users/me", nil)
		reqCtx := middleware.WithClerkUserID(req.Context(), "user_del")
		if userID != 0 {
			reqCtx = middleware.WithUserID(reqCtx, userID)
		}
		rec := httptest.NewRecorder()
		userHandlers.UsersMeAPI(rec, req.WithContext(reqCtx))
		return rec
	}

	// The Clerk call fails after local data and the tombstone are committed.
	if rec := deleteRequest(user.ID); rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 when Clerk deletion fails, got %d body=%s", rec.Code, rec.Body.String())
	}
	if remaining, _ := mockDB.GetUserByID(ctx, user.ID); remaining != nil {
		t.Fatalf("expected local profile to be deleted before the Clerk call")
	}

	// An old, still-valid token for the subject cannot recreate the profile.
	resolution, err := resolver.Resolve(ctx, "user_del")
	if err != nil || !resolution.Deleted || resolution.UserID != 0 {
		t.Fatalf("expected tombstoned resolution, got %+v err=%v", resolution, err)
	}

	// The retry carries only the Clerk subject and finishes Clerk cleanup.
	if rec := deleteRequest(resolution.UserID); rec.Code != http.StatusNoContent {
		t.Fatalf("expected retry to return 204, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(directory.deleted) != 1 || directory.deleted[0] != "user_del" {
		t.Fatalf("expected Clerk user deletion on retry, got %v", directory.deleted)
	}
	if deleted, _ := mockDB.IsClerkUserDeleted(ctx, "user_del"); !deleted {
		t.Fatalf("tombstone must persist after Clerk deletion")
	}
}

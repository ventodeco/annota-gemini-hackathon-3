package handlers_test

import (
	"context"
	"encoding/json"
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
	"github.com/gemini-hackathon/app/internal/testutil"
)

func TestTokenService(t *testing.T) {
	tokenService := auth.NewTokenService("test-secret-key", 30)

	t.Run("GenerateToken", func(t *testing.T) {
		token, expiresAt, err := tokenService.GenerateToken(1)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}
		if token == "" {
			t.Error("Token should not be empty")
		}
		if expiresAt.IsZero() {
			t.Error("ExpiresAt should not be zero")
		}
		expectedExpiry := time.Now().Add(30 * time.Minute)
		if expiresAt.Before(expectedExpiry.Add(-time.Second)) || expiresAt.After(expectedExpiry.Add(time.Second)) {
			t.Errorf("ExpiresAt should be around 30 minutes from now, got %v", expiresAt)
		}
	})

	t.Run("ValidateToken", func(t *testing.T) {
		token, _, _ := tokenService.GenerateToken(42)
		userID, err := tokenService.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}
		if userID != 42 {
			t.Errorf("Expected user ID 42, got %d", userID)
		}
	})

	t.Run("ValidateInvalidToken", func(t *testing.T) {
		_, err := tokenService.ValidateToken("invalid-token")
		if err == nil {
			t.Error("Expected error for invalid token")
		}
	})

	t.Run("ValidateTokenWithWrongSecret", func(t *testing.T) {
		token, _, _ := tokenService.GenerateToken(1)
		otherService := auth.NewTokenService("different-secret", 30)
		_, err := otherService.ValidateToken(token)
		if err == nil {
			t.Error("Expected error when validating token with different secret")
		}
	})
}

func TestGetUserIDMiddleware(t *testing.T) {
	tokenService := auth.NewTokenService("test-secret", 30)
	authMiddleware := middleware.NewAuthMiddleware(tokenService)

	t.Run("WithValidToken", func(t *testing.T) {
		token, _, _ := tokenService.GenerateToken(123)

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("x-token", token)

		var gotUserID int64
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotUserID = middleware.GetUserID(r.Context())
		})

		authMiddleware.Handle(handler).ServeHTTP(httptest.NewRecorder(), req)

		if gotUserID != 123 {
			t.Errorf("Expected user ID 123, got %d", gotUserID)
		}
	})

	t.Run("WithoutToken", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)

		handler := authMiddleware.Handle(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Handler should not be called without token")
		}))

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
	})

	t.Run("WithInvalidToken", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("x-token", "invalid-token")

		handler := authMiddleware.Handle(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Handler should not be called with invalid token")
		}))

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
	})

	t.Run("WithBearerToken", func(t *testing.T) {
		token, _, _ := tokenService.GenerateToken(456)

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		var gotUserID int64
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotUserID = middleware.GetUserID(r.Context())
		})

		authMiddleware.Handle(handler).ServeHTTP(httptest.NewRecorder(), req)

		if gotUserID != 456 {
			t.Errorf("Expected user ID 456, got %d", gotUserID)
		}
	})
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

func TestAuthMiddlewareWithXToken(t *testing.T) {
	tokenService := auth.NewTokenService("test-secret", 30)
	authMiddleware := middleware.NewAuthMiddleware(tokenService)

	t.Run("XTokenPriorityOverBearer", func(t *testing.T) {
		xToken, _, _ := tokenService.GenerateToken(999)

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("x-token", xToken)
		req.Header.Set("Authorization", "Bearer different-token")

		var gotUserID int64
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotUserID = middleware.GetUserID(r.Context())
		})

		authMiddleware.Handle(handler).ServeHTTP(httptest.NewRecorder(), req)

		if gotUserID != 999 {
			t.Errorf("Expected user ID 999 from x-token, got %d", gotUserID)
		}
	})
}

func TestScanHandlers(t *testing.T) {
	mockDB := testutil.NewMockDB()
	cfg := &config.Config{DefaultPageSize: 20, MaxUploadSize: 10 * 1024 * 1024}
	scanHandlers := handlers.NewScanHandlers(mockDB, nil, nil, cfg)

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

package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"testing"

	"github.com/gemini-hackathon/app/internal/ai"
	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/handlers"
	"github.com/gemini-hackathon/app/internal/knowledge"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/testutil"
)

type mockFileStorage struct{}

func (m *mockFileStorage) SaveImage(scanID string, data []byte, mimeType string) (string, *string, error) {
	return "data/uploads/" + scanID + ".jpg", nil, nil
}

func (m *mockFileStorage) OpenImage(path string) ([]byte, error) {
	return []byte("private image"), nil
}

func (m *mockFileStorage) DeleteImage(path string) error {
	return nil
}

func (m *mockFileStorage) SavePDF(documentID string, data []byte) (string, error) {
	return "data/uploads/documents/" + documentID + ".pdf", nil
}

func (m *mockFileStorage) OpenPDF(path string) ([]byte, error) {
	return nil, nil
}

func (m *mockFileStorage) DeletePDF(path string) error {
	return nil
}

type mockAIClient struct{}

func (m *mockAIClient) OCR(ctx context.Context, imageData []byte, mimeType string) (*ai.OCRResponse, error) {
	return &ai.OCRResponse{
		RawText:        "OCR text",
		StructuredJSON: "{}",
		Language:       "JP",
	}, nil
}

func (m *mockAIClient) Annotate(ctx context.Context, ocrText string, selectedText string) (*ai.AnnotationResponse, error) {
	return nil, nil
}

func (m *mockAIClient) AnnotateWithKnowledge(ctx context.Context, ocrText string, selectedText string, entries []knowledge.Entry) (*ai.AnnotationResponse, error) {
	return nil, nil
}

func (m *mockAIClient) SynthesizeSpeech(ctx context.Context, highlightedText string, contextText string) (*ai.SpeechResponse, error) {
	return nil, nil
}

func buildUploadRequest(t *testing.T, path string) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fileHeader := make(textproto.MIMEHeader)
	fileHeader.Set("Content-Disposition", `form-data; name="image"; filename="scan.jpg"`)
	fileHeader.Set("Content-Type", "image/jpeg")
	part, err := writer.CreatePart(fileHeader)
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	if _, err := part.Write([]byte("fake image")); err != nil {
		t.Fatalf("part.Write failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestCreateScanPersistsImageURLAndGetScanReturnsIt(t *testing.T) {
	mockDB := testutil.NewMockDB()
	cfg := &config.Config{MaxUploadSize: 10 * 1024 * 1024}
	scanHandlers := handlers.NewScanHandlers(mockDB, &mockFileStorage{}, &mockAIClient{}, cfg)

	createReq := buildUploadRequest(t, "/v1/scans")
	createReq = createReq.WithContext(middleware.WithUserID(createReq.Context(), 1))
	createRec := httptest.NewRecorder()

	scanHandlers.CreateScanAPI(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	var created handlers.CreateScanResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}
	if created.ImageURL != "/v1/scans/1/image" {
		t.Fatalf("expected created image URL /v1/scans/1/image, got %q", created.ImageURL)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/scans/"+strconv.FormatInt(created.ScanID, 10), nil)
	getReq = getReq.WithContext(middleware.WithUserID(getReq.Context(), 1))
	getRec := httptest.NewRecorder()

	scanHandlers.GetScanAPI(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}

	var scan handlers.GetScanResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &scan); err != nil {
		t.Fatalf("failed to decode get response: %v", err)
	}
	if scan.ImageURL != "/v1/scans/1/image" {
		t.Fatalf("expected persisted image URL /v1/scans/1/image, got %q", scan.ImageURL)
	}
}

func TestGetScanImageRequiresOwnershipAndServesStoredImage(t *testing.T) {
	mockDB := testutil.NewMockDB()
	cfg := &config.Config{MaxUploadSize: 10 * 1024 * 1024, UploadDir: "data/uploads"}
	scanHandlers := handlers.NewScanHandlers(mockDB, &mockFileStorage{}, &mockAIClient{}, cfg)

	createReq := buildUploadRequest(t, "/v1/scans")
	createReq = createReq.WithContext(middleware.WithUserID(createReq.Context(), 1))
	createRec := httptest.NewRecorder()
	scanHandlers.CreateScanAPI(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	imageReq := httptest.NewRequest(http.MethodGet, "/v1/scans/1/image", nil)
	imageReq = imageReq.WithContext(middleware.WithUserID(imageReq.Context(), 1))
	imageRec := httptest.NewRecorder()
	scanHandlers.GetScanAPI(imageRec, imageReq)
	if imageRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", imageRec.Code, imageRec.Body.String())
	}
	if contentType := imageRec.Header().Get("Content-Type"); contentType != "image/jpeg" {
		t.Fatalf("expected image/jpeg content type, got %q", contentType)
	}
	if body := imageRec.Body.String(); body != "private image" {
		t.Fatalf("expected private image body, got %q", body)
	}

	otherUserReq := httptest.NewRequest(http.MethodGet, "/v1/scans/1/image", nil)
	otherUserReq = otherUserReq.WithContext(middleware.WithUserID(otherUserReq.Context(), 2))
	otherUserRec := httptest.NewRecorder()
	scanHandlers.GetScanAPI(otherUserRec, otherUserReq)
	if otherUserRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for another user, got %d", otherUserRec.Code)
	}
}

func TestCreateScanUnauthorized(t *testing.T) {
	mockDB := testutil.NewMockDB()
	cfg := &config.Config{MaxUploadSize: 10 * 1024 * 1024}
	scanHandlers := handlers.NewScanHandlers(mockDB, &mockFileStorage{}, &mockAIClient{}, cfg)

	req := buildUploadRequest(t, "/v1/scans")
	rec := httptest.NewRecorder()

	scanHandlers.CreateScanAPI(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

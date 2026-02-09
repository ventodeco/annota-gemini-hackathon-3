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

	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/gemini"
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
	return nil, nil
}

func (m *mockFileStorage) DeleteImage(path string) error {
	return nil
}

type mockGeminiClient struct{}

func (m *mockGeminiClient) OCR(ctx context.Context, imageData []byte, mimeType string) (*gemini.OCRResponse, error) {
	return &gemini.OCRResponse{
		RawText:        "OCR text",
		StructuredJSON: "{}",
		Language:       "JP",
	}, nil
}

func (m *mockGeminiClient) Annotate(ctx context.Context, ocrText string, selectedText string) (*gemini.AnnotationResponse, error) {
	return nil, nil
}

func (m *mockGeminiClient) AnnotateWithKnowledge(ctx context.Context, ocrText string, selectedText string, entries []knowledge.Entry) (*gemini.AnnotationResponse, error) {
	return nil, nil
}

func (m *mockGeminiClient) SynthesizeSpeech(ctx context.Context, highlightedText string, contextText string) (*gemini.SpeechResponse, error) {
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
	scanHandlers := handlers.NewScanHandlers(mockDB, &mockFileStorage{}, &mockGeminiClient{}, cfg)

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
	if created.ImageURL != "/uploads/1.jpg" {
		t.Fatalf("expected created image URL /uploads/1.jpg, got %q", created.ImageURL)
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
	if scan.ImageURL != "/uploads/1.jpg" {
		t.Fatalf("expected persisted image URL /uploads/1.jpg, got %q", scan.ImageURL)
	}
}

func TestCreateScanUnauthorized(t *testing.T) {
	mockDB := testutil.NewMockDB()
	cfg := &config.Config{MaxUploadSize: 10 * 1024 * 1024}
	scanHandlers := handlers.NewScanHandlers(mockDB, &mockFileStorage{}, &mockGeminiClient{}, cfg)

	req := buildUploadRequest(t, "/v1/scans")
	rec := httptest.NewRecorder()

	scanHandlers.CreateScanAPI(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

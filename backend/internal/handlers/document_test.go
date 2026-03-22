package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/handlers"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/testutil"
)

type mockPDFExtractor struct {
	pages     int
	pageTexts map[int]string
	err       error
}

func (m *mockPDFExtractor) PageCount(filePath string) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.pages, nil
}

func (m *mockPDFExtractor) ExtractText(filePath string, pageNumber int) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if pageNumber < 1 || pageNumber > m.pages {
		return "", fmt.Errorf("page %d out of range", pageNumber)
	}
	return m.pageTexts[pageNumber], nil
}

func newDocumentHandlers(t *testing.T) (*handlers.DocumentHandlers, *testutil.MockDB) {
	t.Helper()
	mockDB := testutil.NewMockDB()
	cfg := &config.Config{MaxUploadSize: 10 * 1024 * 1024}
	pdfExtractor := &mockPDFExtractor{pages: 5, pageTexts: map[int]string{1: "page 1 text"}}
	h := handlers.NewDocumentHandlers(mockDB, &mockFileStorage{}, pdfExtractor, cfg)
	return h, mockDB
}

func buildPDFUploadRequest(t *testing.T, path string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fileHeader := make(textproto.MIMEHeader)
	fileHeader.Set("Content-Disposition", `form-data; name="file"; filename="test.pdf"`)
	fileHeader.Set("Content-Type", "application/pdf")
	part, err := writer.CreatePart(fileHeader)
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("fake pdf content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestUploadDocumentSuccess(t *testing.T) {
	h, _ := newDocumentHandlers(t)
	req := buildPDFUploadRequest(t, "/v1/documents")
	req = req.WithContext(middleware.WithUserID(req.Context(), 1))
	rec := httptest.NewRecorder()

	h.DocumentsAPI(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp handlers.UploadDocumentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.DocumentID == 0 {
		t.Fatal("expected non-zero documentId")
	}
	if resp.PageCount != 5 {
		t.Fatalf("expected pageCount=5, got %d", resp.PageCount)
	}
	if resp.Filename != "test.pdf" {
		t.Fatalf("expected filename=test.pdf, got %s", resp.Filename)
	}
}

func TestUploadDocumentUnauthorized(t *testing.T) {
	h, _ := newDocumentHandlers(t)
	req := buildPDFUploadRequest(t, "/v1/documents")
	// No userID in context
	rec := httptest.NewRecorder()

	h.DocumentsAPI(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestUploadDocumentInvalidMimeType(t *testing.T) {
	h, _ := newDocumentHandlers(t)
	// Build request with image/jpeg instead of application/pdf
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fileHeader := make(textproto.MIMEHeader)
	fileHeader.Set("Content-Disposition", `form-data; name="file"; filename="test.jpg"`)
	fileHeader.Set("Content-Type", "image/jpeg")
	part, _ := writer.CreatePart(fileHeader)
	part.Write([]byte("fake jpeg content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/documents", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = req.WithContext(middleware.WithUserID(req.Context(), 1))
	rec := httptest.NewRecorder()

	h.DocumentsAPI(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUploadDocumentMethodNotAllowed(t *testing.T) {
	h, _ := newDocumentHandlers(t)
	req := httptest.NewRequest(http.MethodDelete, "/v1/documents", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), 1))
	rec := httptest.NewRecorder()

	h.DocumentsAPI(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestGetDocumentSuccess(t *testing.T) {
	h, mockDB := newDocumentHandlers(t)

	mockDB.CreateDocument(context.Background(), &models.Document{
		UserID: 1, FileURL: "test.pdf", Filename: "test.pdf", PageCount: 3, FileSize: 100,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/documents/1", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), 1))
	rec := httptest.NewRecorder()

	h.DocumentByIDAPI(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp handlers.GetDocumentResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Filename != "test.pdf" {
		t.Fatalf("expected test.pdf, got %s", resp.Filename)
	}
	if resp.PageCount != 3 {
		t.Fatalf("expected 3, got %d", resp.PageCount)
	}
}

func TestGetDocumentNotFound(t *testing.T) {
	h, _ := newDocumentHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/documents/999", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), 1))
	rec := httptest.NewRecorder()

	h.DocumentByIDAPI(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestGetPageTextSuccess(t *testing.T) {
	mockDB := testutil.NewMockDB()
	cfg := &config.Config{MaxUploadSize: 10 * 1024 * 1024}
	pdfExtractor := &mockPDFExtractor{
		pages:     3,
		pageTexts: map[int]string{1: "Page one", 2: "Page two", 3: "Page three"},
	}
	h := handlers.NewDocumentHandlers(mockDB, &mockFileStorage{}, pdfExtractor, cfg)

	mockDB.CreateDocument(context.Background(), &models.Document{
		UserID: 1, FileURL: "test.pdf", Filename: "test.pdf", PageCount: 3, FileSize: 100,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/documents/1/pages/2", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), 1))
	rec := httptest.NewRecorder()

	h.DocumentByIDAPI(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp handlers.GetPageResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.PageNumber != 2 {
		t.Fatalf("expected pageNumber=2, got %d", resp.PageNumber)
	}
	if resp.Text != "Page two" {
		t.Fatalf("expected 'Page two', got %q", resp.Text)
	}
	if resp.TotalPages != 3 {
		t.Fatalf("expected totalPages=3, got %d", resp.TotalPages)
	}
}

func TestGetPageTextOutOfRange(t *testing.T) {
	h, mockDB := newDocumentHandlers(t)
	mockDB.CreateDocument(context.Background(), &models.Document{
		UserID: 1, FileURL: "test.pdf", Filename: "test.pdf", PageCount: 3, FileSize: 100,
	})

	tests := []struct {
		name string
		page string
	}{
		{"zero", "0"},
		{"negative", "-1"},
		{"beyond total", "99"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/documents/1/pages/"+tt.page, nil)
			req = req.WithContext(middleware.WithUserID(req.Context(), 1))
			rec := httptest.NewRecorder()
			h.DocumentByIDAPI(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestCreateScanFromPageSuccess(t *testing.T) {
	mockDB := testutil.NewMockDB()
	cfg := &config.Config{MaxUploadSize: 10 * 1024 * 1024}
	pdfExtractor := &mockPDFExtractor{
		pages:     3,
		pageTexts: map[int]string{1: "Page one", 2: "Page two", 3: "Page three"},
	}
	h := handlers.NewDocumentHandlers(mockDB, &mockFileStorage{}, pdfExtractor, cfg)

	mockDB.CreateDocument(context.Background(), &models.Document{
		UserID: 1, FileURL: "test.pdf", Filename: "test.pdf", PageCount: 3, FileSize: 100,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/documents/1/pages/2/scan", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), 1))
	rec := httptest.NewRecorder()

	h.DocumentByIDAPI(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp handlers.CreateScanFromPageResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.ScanID == 0 {
		t.Fatal("expected non-zero scanId")
	}
}

func TestCreateScanFromPageIdempotent(t *testing.T) {
	mockDB := testutil.NewMockDB()
	cfg := &config.Config{MaxUploadSize: 10 * 1024 * 1024}
	pdfExtractor := &mockPDFExtractor{
		pages:     3,
		pageTexts: map[int]string{1: "Page one", 2: "Page two"},
	}
	h := handlers.NewDocumentHandlers(mockDB, &mockFileStorage{}, pdfExtractor, cfg)

	mockDB.CreateDocument(context.Background(), &models.Document{
		UserID: 1, FileURL: "test.pdf", Filename: "test.pdf", PageCount: 3, FileSize: 100,
	})

	// First call
	req1 := httptest.NewRequest(http.MethodPost, "/v1/documents/1/pages/2/scan", nil)
	req1 = req1.WithContext(middleware.WithUserID(req1.Context(), 1))
	rec1 := httptest.NewRecorder()
	h.DocumentByIDAPI(rec1, req1)

	var resp1 handlers.CreateScanFromPageResponse
	json.Unmarshal(rec1.Body.Bytes(), &resp1)

	// Second call — same doc+page should return same scan
	req2 := httptest.NewRequest(http.MethodPost, "/v1/documents/1/pages/2/scan", nil)
	req2 = req2.WithContext(middleware.WithUserID(req2.Context(), 1))
	rec2 := httptest.NewRecorder()
	h.DocumentByIDAPI(rec2, req2)

	var resp2 handlers.CreateScanFromPageResponse
	json.Unmarshal(rec2.Body.Bytes(), &resp2)

	if resp1.ScanID != resp2.ScanID {
		t.Fatalf("expected same scanId, got %d and %d", resp1.ScanID, resp2.ScanID)
	}
	// Second call returns 200 (existing), not 201
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 for idempotent call, got %d", rec2.Code)
	}
}

func TestCreateScanFromPageOutOfRange(t *testing.T) {
	h, mockDB := newDocumentHandlers(t)
	mockDB.CreateDocument(context.Background(), &models.Document{
		UserID: 1, FileURL: "test.pdf", Filename: "test.pdf", PageCount: 3, FileSize: 100,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/documents/1/pages/99/scan", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), 1))
	rec := httptest.NewRecorder()

	h.DocumentByIDAPI(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

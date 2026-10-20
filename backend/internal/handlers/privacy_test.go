package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/handlers"
	"github.com/gemini-hackathon/app/internal/knowledge"
	"github.com/gemini-hackathon/app/internal/logger"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/storage"
	"github.com/gemini-hackathon/app/internal/testutil"
	"github.com/gemini-hackathon/app/internal/usage"
)

// Private markers seeded for account B; none may appear in A's responses.
const (
	ownerOCRText   = "B-private-ocr-text"
	ownerImage     = "B-private-image-bytes"
	ownerPDF       = "B-private-pdf-bytes"
	ownerFilename  = "B-private-book.pdf"
	ownerPageText  = "B-private-page-text"
	ownerHighlight = "B-private-highlight"
)

// faultyFileStorage wraps real local storage and fails selected operations.
type faultyFileStorage struct {
	storage.FileStorage
	failSaveImage   bool
	failDeleteImage int
	failDeletePDF   int
}

func (f *faultyFileStorage) SaveImage(scanID string, data []byte, mimeType string) (string, *string, error) {
	if f.failSaveImage {
		// Simulate a write that left a partial file behind before failing.
		if _, _, err := f.FileStorage.SaveImage(scanID, data[:1], mimeType); err != nil {
			return "", nil, err
		}
		return "", nil, errors.New("disk full")
	}
	return f.FileStorage.SaveImage(scanID, data, mimeType)
}

func (f *faultyFileStorage) DeleteImage(path string) error {
	if f.failDeleteImage > 0 {
		f.failDeleteImage--
		return errors.New("permission denied")
	}
	return f.FileStorage.DeleteImage(path)
}

func (f *faultyFileStorage) DeletePDF(path string) error {
	if f.failDeletePDF > 0 {
		f.failDeletePDF--
		return errors.New("permission denied")
	}
	return f.FileStorage.DeletePDF(path)
}

type privacyFixture struct {
	meter     *usage.Meter
	db        *testutil.MockDB
	files     *faultyFileStorage
	cfg       *config.Config
	extractor *mockPDFExtractor
}

func newPrivacyFixture(t *testing.T) *privacyFixture {
	t.Helper()
	uploadDir := t.TempDir()
	local, err := storage.NewLocalFileStorage(uploadDir)
	if err != nil {
		t.Fatalf("NewLocalFileStorage: %v", err)
	}
	return &privacyFixture{
		meter:     newTestMeter(t),
		db:        testutil.NewMockDB(),
		files:     &faultyFileStorage{FileStorage: local},
		cfg:       &config.Config{MaxUploadSize: 10 * 1024 * 1024, UploadDir: uploadDir, DefaultPageSize: 20},
		extractor: &mockPDFExtractor{pages: 2, pageTexts: map[int]string{1: ownerPageText}},
	}
}

func (f *privacyFixture) scanHandlers() *handlers.ScanHandlers {
	return handlers.NewScanHandlers(f.db, f.files, &mockAIClient{}, f.cfg, f.meter)
}

func (f *privacyFixture) documentHandlers() *handlers.DocumentHandlers {
	return handlers.NewDocumentHandlers(f.db, f.files, f.extractor, f.cfg)
}

func (f *privacyFixture) imagePath(scanID int64, ext string) string {
	return filepath.Join(f.cfg.UploadDir, strconv.FormatInt(scanID, 10)+ext)
}

func (f *privacyFixture) pdfPath(docID int64) string {
	return filepath.Join(f.cfg.UploadDir, "documents", strconv.FormatInt(docID, 10)+".pdf")
}

// seedImageScan stores an image scan for userID with its file at the
// deterministic path, leaving image_url empty when withURL is false.
func (f *privacyFixture) seedImageScan(t *testing.T, userID int64, withURL bool) int64 {
	t.Helper()
	text := ownerOCRText
	scanID, err := f.db.CreateScan(context.Background(), &models.Scan{
		UserID: userID, FullOCRText: &text, SourceType: "image", Status: "ready", CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("CreateScan: %v", err)
	}
	if withURL {
		if err := f.db.UpdateScanImageURL(context.Background(), scanID, "/v1/scans/"+strconv.FormatInt(scanID, 10)+"/image"); err != nil {
			t.Fatalf("UpdateScanImageURL: %v", err)
		}
	}
	if _, _, err := f.files.FileStorage.SaveImage(strconv.FormatInt(scanID, 10), []byte(ownerImage), "image/png"); err != nil {
		t.Fatalf("SaveImage: %v", err)
	}
	return scanID
}

// seedDocument stores a PDF for userID; fileURL stays empty when withURL is false,
// as for uploads that failed before the record was updated.
func (f *privacyFixture) seedDocument(t *testing.T, userID int64, withURL bool) int64 {
	t.Helper()
	docID, err := f.db.CreateDocument(context.Background(), &models.Document{
		UserID: userID, Filename: ownerFilename, CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	path, err := f.files.FileStorage.SavePDF(strconv.FormatInt(docID, 10), []byte(ownerPDF))
	if err != nil {
		t.Fatalf("SavePDF: %v", err)
	}
	if withURL {
		if err := f.db.UpdateDocumentFileInfo(context.Background(), docID, path, f.extractor.pages); err != nil {
			t.Fatalf("UpdateDocumentFileInfo: %v", err)
		}
	}
	return docID
}

func (f *privacyFixture) seedPageScan(t *testing.T, userID, docID int64) int64 {
	t.Helper()
	text, page := ownerPageText, 1
	scanID, err := f.db.CreateScanFromDocument(context.Background(), &models.Scan{
		UserID: userID, FullOCRText: &text, DocumentID: &docID, PageNumber: &page, CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("CreateScanFromDocument: %v", err)
	}
	return scanID
}

func (f *privacyFixture) seedAnnotation(t *testing.T, userID int64, scanID, docID *int64) int64 {
	t.Helper()
	annotationID, err := f.db.CreateAnnotation(context.Background(), &models.Annotation{
		UserID: userID, ScanID: scanID, DocumentID: docID, HighlightedText: ownerHighlight,
		NuanceData: models.NuanceData{Meaning: ownerHighlight}, CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("CreateAnnotation: %v", err)
	}
	return annotationID
}

func requestAs(userID int64, method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req.WithContext(middleware.WithUserID(req.Context(), userID))
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s to exist: %v", path, err)
	}
}

func assertFileGone(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected file %s to be removed, stat err=%v", path, err)
	}
}

func TestCrossAccountRequestsCannotReadOrChangeOwnerData(t *testing.T) {
	f := newPrivacyFixture(t)
	ctx := context.Background()
	const intruder, owner = int64(1), int64(2)
	for _, email := range []string{"a@example.com", "b@example.com"} {
		if err := f.db.CreateUser(ctx, &models.User{Email: email, Provider: "clerk", ProviderID: email}); err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
	}

	scanID := f.seedImageScan(t, owner, true)
	docID := f.seedDocument(t, owner, true)
	pageScanID := f.seedPageScan(t, owner, docID)
	annotationID := f.seedAnnotation(t, owner, &scanID, &docID)

	scans := f.scanHandlers()
	docs := f.documentHandlers()
	annotations := handlers.NewAnnotationHandlers(f.db, f.cfg)
	aiHandlers := handlers.NewAIHandlers(f.db, &mockAIClient{}, knowledge.NewEmptyService(), f.meter)

	scan := "/v1/scans/" + strconv.FormatInt(scanID, 10)
	doc := "/v1/documents/" + strconv.FormatInt(docID, 10)
	annotation := "/v1/annotations/" + strconv.FormatInt(annotationID, 10)
	scanRef := strconv.FormatInt(scanID, 10)

	tests := []struct {
		name    string
		handler http.HandlerFunc
		method  string
		target  string
		body    string
		// listing routes succeed but must contain none of B's data
		wantOK bool
	}{
		{"get scan", scans.GetScanAPI, http.MethodGet, scan, "", false},
		{"get scan image", scans.GetScanAPI, http.MethodGet, scan + "/image", "", false},
		{"delete scan", scans.GetScanAPI, http.MethodDelete, scan, "", false},
		{"list scans", scans.ScansAPI, http.MethodGet, "/v1/scans", "", true},
		{"get document", docs.DocumentByIDAPI, http.MethodGet, doc, "", false},
		{"get document file", docs.DocumentByIDAPI, http.MethodGet, doc + "/file", "", false},
		{"get document page", docs.DocumentByIDAPI, http.MethodGet, doc + "/pages/1", "", false},
		{"create page scan", docs.DocumentByIDAPI, http.MethodPost, doc + "/pages/1/scan", "", false},
		{"update progress", docs.DocumentByIDAPI, http.MethodPatch, doc + "/progress", `{"lastPageNumber":2}`, false},
		{"delete document", docs.DocumentByIDAPI, http.MethodDelete, doc, "", false},
		{"list documents", docs.DocumentsAPI, http.MethodGet, "/v1/documents", "", true},
		{"get annotation", annotations.AnnotationByIDAPI, http.MethodGet, annotation, "", false},
		{"delete annotation", annotations.AnnotationByIDAPI, http.MethodDelete, annotation, "", false},
		{"list annotations", annotations.AnnotationsAPI, http.MethodGet, "/v1/annotations", "", true},
		{"list annotations by scan", annotations.AnnotationsAPI, http.MethodGet, "/v1/annotations?scanId=" + scanRef, "", true},
		{"list annotations by document", annotations.AnnotationsAPI, http.MethodGet, "/v1/annotations?documentId=" + strconv.FormatInt(docID, 10), "", true},
		{"annotate owner scan", annotations.AnnotationsAPI, http.MethodPost, "/v1/annotations",
			`{"scanId":` + scanRef + `,"highlightedText":"x","nuanceData":{"meaning":"x"}}`, false},
		{"analyze with owner scan context", aiHandlers.AnalyzeAPI, http.MethodPost, "/v1/ai/analyze",
			`{"textToAnalyze":"x","scanId":` + scanRef + `}`, false},
	}

	privateMarkers := []string{ownerOCRText, ownerImage, ownerPDF, ownerFilename, ownerPageText, ownerHighlight}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tt.handler(rec, requestAs(intruder, tt.method, tt.target, tt.body))

			success := rec.Code >= 200 && rec.Code < 300
			if success != tt.wantOK {
				t.Fatalf("status = %d, want success=%v body=%s", rec.Code, tt.wantOK, rec.Body.String())
			}
			for _, marker := range privateMarkers {
				if strings.Contains(rec.Body.String(), marker) {
					t.Fatalf("response leaked %q: %s", marker, rec.Body.String())
				}
			}
		})
	}

	// Every owner row and file is untouched, checked by stable ID and path.
	if got, _ := f.db.GetScanByID(ctx, scanID); got == nil || got.UserID != owner {
		t.Fatalf("owner scan %d changed: %+v", scanID, got)
	}
	if got, _ := f.db.GetScanByID(ctx, pageScanID); got == nil || got.UserID != owner {
		t.Fatalf("owner page scan %d changed: %+v", pageScanID, got)
	}
	gotDoc, _ := f.db.GetDocumentByID(ctx, docID)
	if gotDoc == nil || gotDoc.UserID != owner || gotDoc.LastPageNumber != 1 {
		t.Fatalf("owner document %d changed: %+v", docID, gotDoc)
	}
	if got, _ := f.db.GetAnnotationByID(ctx, annotationID); got == nil || got.UserID != owner {
		t.Fatalf("owner annotation %d changed: %+v", annotationID, got)
	}
	if got, _ := f.db.GetAnnotationsByUserID(ctx, intruder, 1, 100); len(got) != 0 {
		t.Fatalf("intruder gained annotations: %+v", got)
	}
	assertFileExists(t, f.imagePath(scanID, ".png"))
	assertFileExists(t, f.pdfPath(docID))
}

func TestScanUploadFailureLeavesNoRecordOrFile(t *testing.T) {
	f := newPrivacyFixture(t)
	f.files.failSaveImage = true

	req := buildUploadRequest(t, "/v1/scans")
	req = req.WithContext(middleware.WithUserID(req.Context(), 1))
	rec := httptest.NewRecorder()
	f.scanHandlers().CreateScanAPI(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 body=%s", rec.Code, rec.Body.String())
	}
	if got, _ := f.db.GetScanByID(context.Background(), 1); got != nil {
		t.Fatalf("failed upload left scan record: %+v", got)
	}
	assertFileGone(t, f.imagePath(1, ".jpg"))

	// No OCR runs for a discarded upload, so its reserved page is refunded.
	snapshot, err := f.meter.Snapshot(context.Background(), 1)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	for _, feature := range snapshot.Features {
		if feature.Feature == usage.FeatureOCR && feature.Used != 0 {
			t.Fatalf("OCR usage after a discarded upload = %d, want 0", feature.Used)
		}
	}
}

func TestMalformedPDFUploadLeavesNoRecordOrFile(t *testing.T) {
	f := newPrivacyFixture(t)
	f.extractor.err = errors.New("malformed PDF")

	req := buildPDFUploadRequest(t, "/v1/documents")
	req = req.WithContext(middleware.WithUserID(req.Context(), 1))
	rec := httptest.NewRecorder()
	f.documentHandlers().DocumentsAPI(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 body=%s", rec.Code, rec.Body.String())
	}
	if got, _ := f.db.GetDocumentByID(context.Background(), 1); got != nil {
		t.Fatalf("malformed upload left document record: %+v", got)
	}
	assertFileGone(t, f.pdfPath(1))
}

func TestScanDeleteKeepsRecordUntilImageIsRemoved(t *testing.T) {
	f := newPrivacyFixture(t)
	ctx := context.Background()
	// image_url empty, as when recording the URL failed after the file was saved.
	scanID := f.seedImageScan(t, 1, false)
	annotationID := f.seedAnnotation(t, 1, &scanID, nil)
	f.files.failDeleteImage = 1
	target := "/v1/scans/" + strconv.FormatInt(scanID, 10)

	rec := httptest.NewRecorder()
	f.scanHandlers().GetScanAPI(rec, requestAs(1, http.MethodDelete, target, ""))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("first delete status = %d, want 500", rec.Code)
	}
	if got, _ := f.db.GetScanByID(ctx, scanID); got == nil {
		t.Fatal("scan record removed although its image is still stored")
	}

	rec = httptest.NewRecorder()
	f.scanHandlers().GetScanAPI(rec, requestAs(1, http.MethodDelete, target, ""))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("retry status = %d, want 204 body=%s", rec.Code, rec.Body.String())
	}
	if got, _ := f.db.GetScanByID(ctx, scanID); got != nil {
		t.Fatalf("scan %d still stored after delete", scanID)
	}
	assertFileGone(t, f.imagePath(scanID, ".png"))
	// Saved annotations are separate learning items and outlive their source.
	if got, _ := f.db.GetAnnotationByID(ctx, annotationID); got == nil || got.HighlightedText != ownerHighlight {
		t.Fatalf("annotation %d should be retained after scan deletion, got %+v", annotationID, got)
	}
}

func TestDocumentDeleteRemovesFileThenPageScansAndRetries(t *testing.T) {
	f := newPrivacyFixture(t)
	ctx := context.Background()
	docID := f.seedDocument(t, 1, true)
	pageScanID := f.seedPageScan(t, 1, docID)
	annotationID := f.seedAnnotation(t, 1, &pageScanID, &docID)
	f.files.failDeletePDF = 1
	target := "/v1/documents/" + strconv.FormatInt(docID, 10)

	rec := httptest.NewRecorder()
	f.documentHandlers().DocumentByIDAPI(rec, requestAs(1, http.MethodDelete, target, ""))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("first delete status = %d, want 500", rec.Code)
	}
	if got, _ := f.db.GetDocumentByID(ctx, docID); got == nil {
		t.Fatal("document record removed although its PDF is still stored")
	}
	if got, _ := f.db.GetScanByID(ctx, pageScanID); got == nil {
		t.Fatal("page scan removed although the document deletion did not complete")
	}
	assertFileExists(t, f.pdfPath(docID))

	rec = httptest.NewRecorder()
	f.documentHandlers().DocumentByIDAPI(rec, requestAs(1, http.MethodDelete, target, ""))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("retry status = %d, want 204 body=%s", rec.Code, rec.Body.String())
	}
	if got, _ := f.db.GetDocumentByID(ctx, docID); got != nil {
		t.Fatalf("document %d still stored", docID)
	}
	if got, _ := f.db.GetScanByID(ctx, pageScanID); got != nil {
		t.Fatalf("page scan %d with extracted text still stored", pageScanID)
	}
	assertFileGone(t, f.pdfPath(docID))
	if got, _ := f.db.GetAnnotationByID(ctx, annotationID); got == nil {
		t.Fatalf("annotation %d should be retained after document deletion", annotationID)
	}
}

func TestDocumentDeleteRemovesPDFOfRecordWithoutFileURL(t *testing.T) {
	f := newPrivacyFixture(t)
	docID := f.seedDocument(t, 1, false)

	rec := httptest.NewRecorder()
	f.documentHandlers().DocumentByIDAPI(rec, requestAs(1, http.MethodDelete, "/v1/documents/"+strconv.FormatInt(docID, 10), ""))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 body=%s", rec.Code, rec.Body.String())
	}
	assertFileGone(t, f.pdfPath(docID))
}

func TestDeleteAccountRemovesFilesOfIncompleteUploads(t *testing.T) {
	f := newPrivacyFixture(t)
	ctx := context.Background()
	for _, email := range []string{"a@example.com", "b@example.com"} {
		if err := f.db.CreateUser(ctx, &models.User{Email: email, Provider: "clerk", ProviderID: email}); err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
	}
	const deleting, other = int64(1), int64(2)
	scanWithoutURL := f.seedImageScan(t, deleting, false)
	docWithoutURL := f.seedDocument(t, deleting, false)
	docWithURL := f.seedDocument(t, deleting, true)
	otherScan := f.seedImageScan(t, other, true)
	otherDoc := f.seedDocument(t, other, true)

	users := handlers.NewUserHandlersWithStorage(f.db, f.files, f.cfg, &fakeIdentityDirectory{})
	req := requestAs(deleting, http.MethodDelete, "/v1/users/me", "")
	req = req.WithContext(middleware.WithClerkUserID(req.Context(), "user_a"))
	rec := httptest.NewRecorder()
	users.UsersMeAPI(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 body=%s", rec.Code, rec.Body.String())
	}

	assertFileGone(t, f.imagePath(scanWithoutURL, ".png"))
	assertFileGone(t, f.pdfPath(docWithoutURL))
	assertFileGone(t, f.pdfPath(docWithURL))
	assertFileExists(t, f.imagePath(otherScan, ".png"))
	assertFileExists(t, f.pdfPath(otherDoc))
	if got, _ := f.db.GetDocumentByID(ctx, otherDoc); got == nil {
		t.Fatalf("other account's document %d was removed", otherDoc)
	}
}

func TestScanImageResponseIsNeverStored(t *testing.T) {
	f := newPrivacyFixture(t)
	scanID := f.seedImageScan(t, 1, true)

	rec := httptest.NewRecorder()
	f.scanHandlers().GetScanAPI(rec, requestAs(1, http.MethodGet, "/v1/scans/"+strconv.FormatInt(scanID, 10)+"/image", ""))
	if rec.Code != http.StatusOK || rec.Body.String() != ownerImage {
		t.Fatalf("owner image status = %d body=%q", rec.Code, rec.Body.String())
	}
	// A cached copy is keyed by URL only: another account on the same browser
	// could be shown it, and it would outlive the scan's deletion.
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}

func TestMalformedPDFParseErrorIsNotLogged(t *testing.T) {
	f := newPrivacyFixture(t)
	// The PDF parser quotes bytes of the file in some errors.
	f.extractor.err = errors.New("malformed PDF: cross-reference table not found: SENTINEL-UPLOADED-BYTES")
	readLogs := captureLogs(t)

	req := buildPDFUploadRequest(t, "/v1/documents")
	req = req.WithContext(middleware.WithUserID(req.Context(), 1))
	rec := httptest.NewRecorder()
	f.documentHandlers().DocumentsAPI(rec, req)

	logged := readLogs()
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(logged, "Uploaded PDF could not be parsed") {
		t.Fatalf("expected a fixed parse-failure log line, got %s", logged)
	}
	if strings.Contains(logged, "SENTINEL-UPLOADED-BYTES") || strings.Contains(rec.Body.String(), "SENTINEL") {
		t.Fatalf("parser error content leaked: log=%s body=%s", logged, rec.Body.String())
	}
}

// pausingFileStorage blocks the first save until released, to hold an upload
// between creating its record and publishing its file.
type pausingFileStorage struct {
	storage.FileStorage
	entered chan struct{}
	release chan struct{}
}

func (p *pausingFileStorage) pause() {
	p.entered <- struct{}{}
	<-p.release
}

func (p *pausingFileStorage) SaveImage(scanID string, data []byte, mimeType string) (string, *string, error) {
	p.pause()
	return p.FileStorage.SaveImage(scanID, data, mimeType)
}

func (p *pausingFileStorage) SavePDF(documentID string, data []byte) (string, error) {
	p.pause()
	return p.FileStorage.SavePDF(documentID, data)
}

func TestAccountDeletionWaitsForInFlightUpload(t *testing.T) {
	tests := []struct {
		name   string
		upload func(f *privacyFixture, files storage.FileStorage) *httptest.ResponseRecorder
		path   func(f *privacyFixture) string
		stored func(f *privacyFixture) bool
	}{
		{
			name: "pdf",
			upload: func(f *privacyFixture, files storage.FileStorage) *httptest.ResponseRecorder {
				req := buildPDFUploadRequest(t, "/v1/documents")
				rec := httptest.NewRecorder()
				handlers.NewDocumentHandlers(f.db, files, f.extractor, f.cfg).DocumentsAPI(rec, req.WithContext(middleware.WithUserID(req.Context(), 1)))
				return rec
			},
			path: func(f *privacyFixture) string { return f.pdfPath(1) },
			stored: func(f *privacyFixture) bool {
				doc, _ := f.db.GetDocumentByID(context.Background(), 1)
				return doc != nil
			},
		},
		{
			name: "image",
			upload: func(f *privacyFixture, files storage.FileStorage) *httptest.ResponseRecorder {
				req := buildUploadRequest(t, "/v1/scans")
				rec := httptest.NewRecorder()
				handlers.NewScanHandlers(f.db, files, &mockAIClient{}, f.cfg, f.meter).CreateScanAPI(rec, req.WithContext(middleware.WithUserID(req.Context(), 1)))
				return rec
			},
			path: func(f *privacyFixture) string { return f.imagePath(1, ".jpg") },
			stored: func(f *privacyFixture) bool {
				scan, _ := f.db.GetScanByID(context.Background(), 1)
				return scan != nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newPrivacyFixture(t)
			if err := f.db.CreateUser(context.Background(), &models.User{Email: "a@example.com", Provider: "clerk", ProviderID: "a"}); err != nil {
				t.Fatalf("CreateUser: %v", err)
			}
			files := &pausingFileStorage{FileStorage: f.files, entered: make(chan struct{}), release: make(chan struct{})}

			uploaded := make(chan *httptest.ResponseRecorder, 1)
			go func() { uploaded <- tt.upload(f, files) }()
			<-files.entered // the record exists; the file is not written yet

			deleted := make(chan *httptest.ResponseRecorder, 1)
			go func() {
				users := handlers.NewUserHandlersWithStorage(f.db, f.files, f.cfg, &fakeIdentityDirectory{})
				req := requestAs(1, http.MethodDelete, "/v1/users/me", "")
				rec := httptest.NewRecorder()
				users.UsersMeAPI(rec, req.WithContext(middleware.WithClerkUserID(req.Context(), "user_a")))
				deleted <- rec
			}()

			select {
			case rec := <-deleted:
				t.Fatalf("account deletion finished (%d) while an upload was still writing its file", rec.Code)
			case <-time.After(100 * time.Millisecond):
			}
			close(files.release)

			if rec := <-uploaded; rec.Code != http.StatusCreated {
				t.Fatalf("upload status = %d, want 201 body=%s", rec.Code, rec.Body.String())
			}
			if rec := <-deleted; rec.Code != http.StatusNoContent {
				t.Fatalf("delete status = %d, want 204 body=%s", rec.Code, rec.Body.String())
			}
			assertFileGone(t, tt.path(f))
			if tt.stored(f) {
				t.Fatal("upload record survived account deletion")
			}
		})
	}
}

// captureLogs redirects the default logger for the test and returns a reader
// for everything logged so far.
func captureLogs(t *testing.T) func() string {
	t.Helper()
	logFile, err := os.CreateTemp(t.TempDir(), "log")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	logger.SetOutput(logFile)
	t.Cleanup(func() { logger.SetOutput(os.Stdout) })
	return func() string {
		logged, err := os.ReadFile(logFile.Name())
		if err != nil {
			t.Fatalf("read log: %v", err)
		}
		return string(logged)
	}
}

func TestMalformedMultipartUploadIsNotLogged(t *testing.T) {
	f := newPrivacyFixture(t)
	const sentinel = "PRIVATE_DOCUMENT_SENTINEL"
	// A part header line without a colon makes the parser quote it in its error.
	body := "--b\r\n" + sentinel + "\r\n\r\ndata\r\n--b--\r\n"

	tests := []struct {
		name    string
		target  string
		handler http.HandlerFunc
	}{
		{"pdf upload", "/v1/documents", f.documentHandlers().DocumentsAPI},
		{"image upload", "/v1/scans", f.scanHandlers().ScansAPI},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readLogs := captureLogs(t)
			req := requestAs(1, http.MethodPost, tt.target, body)
			req.Header.Set("Content-Type", "multipart/form-data; boundary=b")
			rec := httptest.NewRecorder()
			tt.handler(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 body=%s", rec.Code, rec.Body.String())
			}
			logged := readLogs()
			if !strings.Contains(logged, "Failed to parse multipart form") {
				t.Fatalf("expected a fixed parse-failure log line, got %s", logged)
			}
			if strings.Contains(logged, sentinel) || strings.Contains(rec.Body.String(), sentinel) {
				t.Fatalf("request content leaked: log=%s body=%s", logged, rec.Body.String())
			}
		})
	}
}

func TestProfileResponseClearsBrowserHTTPCache(t *testing.T) {
	f := newPrivacyFixture(t)
	if err := f.db.CreateUser(context.Background(), &models.User{Email: "a@example.com", Provider: "clerk", ProviderID: "a"}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	users := handlers.NewUserHandlersWithStorage(f.db, f.files, f.cfg, &fakeIdentityDirectory{})

	rec := httptest.NewRecorder()
	users.UsersMeAPI(rec, requestAs(1, http.MethodGet, "/v1/users/me", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Clear-Site-Data"); got != `"cache"` {
		t.Fatalf("Clear-Site-Data = %q, want \"cache\"", got)
	}
}

func TestSourceDeletionWaitsForInFlightUpload(t *testing.T) {
	tests := []struct {
		name   string
		upload func(f *privacyFixture, files storage.FileStorage) *httptest.ResponseRecorder
		delete func(f *privacyFixture) *httptest.ResponseRecorder
		path   func(f *privacyFixture) string
	}{
		{
			name: "document",
			upload: func(f *privacyFixture, files storage.FileStorage) *httptest.ResponseRecorder {
				req := buildPDFUploadRequest(t, "/v1/documents")
				rec := httptest.NewRecorder()
				handlers.NewDocumentHandlers(f.db, files, f.extractor, f.cfg).DocumentsAPI(rec, req.WithContext(middleware.WithUserID(req.Context(), 1)))
				return rec
			},
			delete: func(f *privacyFixture) *httptest.ResponseRecorder {
				rec := httptest.NewRecorder()
				f.documentHandlers().DocumentByIDAPI(rec, requestAs(1, http.MethodDelete, "/v1/documents/1", ""))
				return rec
			},
			path: func(f *privacyFixture) string { return f.pdfPath(1) },
		},
		{
			name: "scan",
			upload: func(f *privacyFixture, files storage.FileStorage) *httptest.ResponseRecorder {
				req := buildUploadRequest(t, "/v1/scans")
				rec := httptest.NewRecorder()
				handlers.NewScanHandlers(f.db, files, &mockAIClient{}, f.cfg, f.meter).CreateScanAPI(rec, req.WithContext(middleware.WithUserID(req.Context(), 1)))
				return rec
			},
			delete: func(f *privacyFixture) *httptest.ResponseRecorder {
				rec := httptest.NewRecorder()
				f.scanHandlers().GetScanAPI(rec, requestAs(1, http.MethodDelete, "/v1/scans/1", ""))
				return rec
			},
			path: func(f *privacyFixture) string { return f.imagePath(1, ".jpg") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newPrivacyFixture(t)
			files := &pausingFileStorage{FileStorage: f.files, entered: make(chan struct{}), release: make(chan struct{})}

			uploaded := make(chan *httptest.ResponseRecorder, 1)
			go func() { uploaded <- tt.upload(f, files) }()
			<-files.entered // the row exists; its file is not written yet

			deleted := make(chan *httptest.ResponseRecorder, 1)
			go func() { deleted <- tt.delete(f) }()
			select {
			case rec := <-deleted:
				t.Fatalf("deletion finished (%d) while the upload was still writing its file", rec.Code)
			case <-time.After(100 * time.Millisecond):
			}
			close(files.release)

			if rec := <-uploaded; rec.Code != http.StatusCreated {
				t.Fatalf("upload status = %d body=%s", rec.Code, rec.Body.String())
			}
			if rec := <-deleted; rec.Code != http.StatusNoContent {
				t.Fatalf("delete status = %d body=%s", rec.Code, rec.Body.String())
			}
			assertFileGone(t, tt.path(f))
		})
	}
}

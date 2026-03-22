package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/httputil"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/pdf"
	"github.com/gemini-hackathon/app/internal/storage"
)

// DocumentHandlers handles HTTP requests for document (PDF) operations.
type DocumentHandlers struct {
	db           storage.DB
	fileStorage  storage.FileStorage
	pdfExtractor pdf.Extractor
	config       *config.Config
}

// NewDocumentHandlers creates a new DocumentHandlers instance.
func NewDocumentHandlers(db storage.DB, fileStorage storage.FileStorage, pdfExtractor pdf.Extractor, cfg *config.Config) *DocumentHandlers {
	return &DocumentHandlers{
		db:           db,
		fileStorage:  fileStorage,
		pdfExtractor: pdfExtractor,
		config:       cfg,
	}
}

// UploadDocumentResponse is the JSON response for a successful PDF upload.
type UploadDocumentResponse struct {
	DocumentID int64  `json:"documentId"`
	PageCount  int    `json:"pageCount"`
	Filename   string `json:"filename"`
}

// DocumentsAPI dispatches requests to the appropriate handler by HTTP method.
func (h *DocumentHandlers) DocumentsAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.uploadDocumentHandler(w, r)
	default:
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *DocumentHandlers) uploadDocumentHandler(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if err := r.ParseMultipartForm(h.config.MaxUploadSize); err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Failed to parse form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Please select a PDF file to upload")
		return
	}
	defer file.Close()

	mimeType := header.Header.Get("Content-Type")
	if mimeType != "application/pdf" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid file type. Please upload a PDF file.")
		return
	}

	if header.Size > h.config.MaxUploadSize {
		httputil.WriteJSONError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("File too large. Maximum size is %d MB.", h.config.MaxUploadSize/(1024*1024)))
		return
	}

	pdfData, err := io.ReadAll(file)
	if err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Failed to read uploaded file")
		return
	}

	if len(pdfData) == 0 {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Uploaded file is empty")
		return
	}

	now := time.Now()
	doc := &models.Document{
		UserID:    userID,
		FileURL:   "",
		Filename:  header.Filename,
		PageCount: 0,
		FileSize:  int64(len(pdfData)),
		CreatedAt: now,
	}

	docID, err := h.db.CreateDocument(r.Context(), doc)
	if err != nil {
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to create document record")
		return
	}

	filePath, err := h.fileStorage.SavePDF(strconv.FormatInt(docID, 10), pdfData)
	if err != nil {
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to save PDF file")
		return
	}

	pageCount, err := h.pdfExtractor.PageCount(filePath)
	if err != nil {
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to read PDF page count")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, UploadDocumentResponse{
		DocumentID: docID,
		PageCount:  pageCount,
		Filename:   header.Filename,
	})
}

// GetDocumentResponse is the JSON response for retrieving document metadata.
type GetDocumentResponse struct {
	ID        int64  `json:"id"`
	Filename  string `json:"filename"`
	PageCount int    `json:"pageCount"`
	CreatedAt string `json:"createdAt"`
}

// GetPageResponse is the JSON response for retrieving a single page's text.
type GetPageResponse struct {
	PageNumber int    `json:"pageNumber"`
	Text       string `json:"text"`
	TotalPages int    `json:"totalPages"`
}

// CreateScanFromPageResponse is the JSON response for creating a scan from a document page.
type CreateScanFromPageResponse struct {
	ScanID int64 `json:"scanId"`
}

// DocumentByIDAPI routes requests for /v1/documents/{id} and sub-resources.
func (h *DocumentHandlers) DocumentByIDAPI(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	remainder := strings.TrimPrefix(r.URL.Path, "/v1/documents/")
	parts := strings.Split(remainder, "/")

	docID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid document ID")
		return
	}

	doc, err := h.db.GetDocumentByID(r.Context(), docID)
	if err != nil {
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to retrieve document")
		return
	}
	if doc == nil {
		httputil.WriteJSONError(w, http.StatusNotFound, "Document not found")
		return
	}

	if doc.UserID != userID {
		httputil.WriteJSONError(w, http.StatusForbidden, "Access denied")
		return
	}

	switch {
	case len(parts) == 1:
		if r.Method != http.MethodGet {
			httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		h.getDocumentHandler(w, doc)

	case len(parts) == 3 && parts[1] == "pages":
		pageNum, err := strconv.Atoi(parts[2])
		if err != nil {
			httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid page number")
			return
		}
		if r.Method != http.MethodGet {
			httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		h.getPageTextHandler(w, doc, pageNum)

	case len(parts) == 4 && parts[1] == "pages" && parts[3] == "scan":
		pageNum, err := strconv.Atoi(parts[2])
		if err != nil {
			httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid page number")
			return
		}
		if r.Method != http.MethodPost {
			httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		h.createScanFromPageHandler(w, r, doc, pageNum)

	default:
		httputil.WriteJSONError(w, http.StatusNotFound, "Not found")
	}
}

func (h *DocumentHandlers) getDocumentHandler(w http.ResponseWriter, doc *models.Document) {
	httputil.WriteJSON(w, http.StatusOK, GetDocumentResponse{
		ID:        doc.ID,
		Filename:  doc.Filename,
		PageCount: doc.PageCount,
		CreatedAt: doc.CreatedAt.Format(time.RFC3339),
	})
}

func (h *DocumentHandlers) createScanFromPageHandler(w http.ResponseWriter, r *http.Request, doc *models.Document, pageNumber int) {
	userID := middleware.GetUserID(r.Context())

	if pageNumber < 1 || pageNumber > doc.PageCount {
		httputil.WriteJSONError(w, http.StatusNotFound,
			fmt.Sprintf("Page %d not found. Document has %d pages.", pageNumber, doc.PageCount))
		return
	}

	// Idempotency: check if scan already exists for this document+page
	existing, err := h.db.GetScanByDocumentPage(r.Context(), doc.ID, pageNumber)
	if err == nil && existing != nil {
		httputil.WriteJSON(w, http.StatusOK, CreateScanFromPageResponse{ScanID: existing.ID})
		return
	}

	// Extract text from page
	text, err := h.pdfExtractor.ExtractText(doc.FileURL, pageNumber)
	if err != nil {
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to extract text from page")
		return
	}

	// Create scan with text populated immediately
	scan := &models.Scan{
		UserID:     userID,
		FullOCRText: &text,
		DocumentID: &doc.ID,
		PageNumber: &pageNumber,
		CreatedAt:  time.Now(),
	}

	scanID, err := h.db.CreateScanFromDocument(r.Context(), scan)
	if err != nil {
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to create scan")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, CreateScanFromPageResponse{ScanID: scanID})
}

func (h *DocumentHandlers) getPageTextHandler(w http.ResponseWriter, doc *models.Document, pageNumber int) {
	if pageNumber < 1 || pageNumber > doc.PageCount {
		httputil.WriteJSONError(w, http.StatusNotFound, "Page not found")
		return
	}

	text, err := h.pdfExtractor.ExtractText(doc.FileURL, pageNumber)
	if err != nil {
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to extract page text")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, GetPageResponse{
		PageNumber: pageNumber,
		Text:       text,
		TotalPages: doc.PageCount,
	})
}

package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/httputil"
	"github.com/gemini-hackathon/app/internal/logger"
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
	case http.MethodGet:
		h.listDocumentsHandler(w, r)
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

	log := logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context())).WithUserID(userID)

	pdfData, header, err := readFormFile(r, h.config.MaxUploadSize, "file")
	if err != nil {
		switch {
		case errors.Is(err, ErrMultipartParse):
			log.Warnf("Failed to parse multipart form: %v", err)
			httputil.WriteJSONError(w, http.StatusBadRequest, "Failed to parse form")
		case errors.Is(err, ErrMultipartField):
			log.Warnf("Missing file field: %v", err)
			httputil.WriteJSONError(w, http.StatusBadRequest, "Please select a PDF file to upload")
		default:
			log.ErrorWithErr(err, "Failed to read uploaded file")
			httputil.WriteJSONError(w, http.StatusBadRequest, "Failed to read uploaded file")
		}
		return
	}

	mimeType := header.Header.Get("Content-Type")
	if mimeType != "application/pdf" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid file type. Please upload a PDF file.")
		return
	}

	if header.Size > h.config.MaxUploadSize {
		httputil.WriteJSONError(w, http.StatusRequestEntityTooLarge, fileTooLargeMBMessage(h.config.MaxUploadSize))
		return
	}

	if len(pdfData) == 0 {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Uploaded file is empty")
		return
	}

	now := time.Now()
	doc := &models.Document{
		UserID:         userID,
		FileURL:        "",
		Filename:       header.Filename,
		PageCount:      0,
		FileSize:       int64(len(pdfData)),
		LastPageNumber: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
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

	if err := h.db.UpdateDocumentFileInfo(r.Context(), docID, filePath, pageCount); err != nil {
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to update document record")
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
	ID             int64   `json:"id"`
	Filename       string  `json:"filename"`
	PageCount      int     `json:"pageCount"`
	LastPageNumber int     `json:"lastPageNumber"`
	LastOpenedAt   *string `json:"lastOpenedAt,omitempty"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
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

type GetDocumentsResponse struct {
	Data []GetDocumentResponse `json:"data"`
	Meta PaginationMeta        `json:"meta"`
}

type UpdateDocumentProgressRequest struct {
	LastPageNumber int `json:"lastPageNumber"`
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
		switch r.Method {
		case http.MethodGet:
			h.getDocumentHandler(w, doc)
		case http.MethodDelete:
			h.deleteDocumentHandler(w, r, doc)
		default:
			httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}

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

	case len(parts) == 2 && parts[1] == "file":
		if r.Method != http.MethodGet {
			httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		h.getDocumentFileHandler(w, r, doc)

	case len(parts) == 2 && parts[1] == "progress":
		if r.Method != http.MethodPatch {
			httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		h.updateDocumentProgressHandler(w, r, doc)

	default:
		httputil.WriteJSONError(w, http.StatusNotFound, "Not found")
	}
}

func (h *DocumentHandlers) getDocumentHandler(w http.ResponseWriter, doc *models.Document) {
	httputil.WriteJSON(w, http.StatusOK, toDocumentResponse(doc))
}

func (h *DocumentHandlers) listDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	page, size := httputil.ParsePagination(r, h.config.DefaultPageSize)
	docs, err := h.db.GetDocumentsByUserID(r.Context(), userID, page, size)
	if err != nil {
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to retrieve documents")
		return
	}

	data := make([]GetDocumentResponse, len(docs))
	for i, doc := range docs {
		data[i] = toDocumentResponse(doc)
	}

	var nextPage, prevPage *int
	if len(docs) == size {
		nextPageVal := page + 1
		nextPage = &nextPageVal
	}
	if page > 1 {
		prevPageVal := page - 1
		prevPage = &prevPageVal
	}

	httputil.WriteJSON(w, http.StatusOK, GetDocumentsResponse{
		Data: data,
		Meta: PaginationMeta{
			CurrentPage:  page,
			PageSize:     size,
			NextPage:     nextPage,
			PreviousPage: prevPage,
		},
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
		UserID:      userID,
		FullOCRText: &text,
		DocumentID:  &doc.ID,
		PageNumber:  &pageNumber,
		SourceType:  "pdf",
		Status:      "ready",
		CreatedAt:   time.Now(),
	}

	scanID, err := h.db.CreateScanFromDocument(r.Context(), scan)
	if err != nil {
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to create scan")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, CreateScanFromPageResponse{ScanID: scanID})
}

func (h *DocumentHandlers) updateDocumentProgressHandler(w http.ResponseWriter, r *http.Request, doc *models.Document) {
	userID := middleware.GetUserID(r.Context())
	var req UpdateDocumentProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.LastPageNumber < 1 || req.LastPageNumber > doc.PageCount {
		httputil.WriteJSONError(w, http.StatusBadRequest, "lastPageNumber is out of range")
		return
	}
	if err := h.db.UpdateDocumentProgress(r.Context(), doc.ID, userID, req.LastPageNumber); err != nil {
		httputil.WriteJSONError(w, http.StatusNotFound, "Document not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (h *DocumentHandlers) deleteDocumentHandler(w http.ResponseWriter, r *http.Request, doc *models.Document) {
	userID := middleware.GetUserID(r.Context())
	if err := h.db.DeleteScansByDocument(r.Context(), doc.ID, userID); err != nil {
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to delete document scans")
		return
	}
	if err := h.db.DeleteDocument(r.Context(), doc.ID, userID); err != nil {
		httputil.WriteJSONError(w, http.StatusNotFound, "Document not found")
		return
	}
	if err := h.fileStorage.DeletePDF(doc.FileURL); err != nil {
		logger.GetDefaultLogger().
			WithRequestID(middleware.GetRequestID(r.Context())).
			Warnf("Failed to delete PDF file: %v", err)
	}
	w.WriteHeader(http.StatusNoContent)
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

func (h *DocumentHandlers) getDocumentFileHandler(w http.ResponseWriter, r *http.Request, doc *models.Document) {
	data, err := h.fileStorage.OpenPDF(doc.FileURL)
	if err != nil {
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to read PDF file")
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", doc.Filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		logger.GetDefaultLogger().
			WithRequestID(middleware.GetRequestID(r.Context())).
			ErrorWithErr(err, "Failed to write PDF response")
	}
}

func toDocumentResponse(doc *models.Document) GetDocumentResponse {
	var lastOpenedAt *string
	if doc.LastOpenedAt != nil {
		value := doc.LastOpenedAt.Format(time.RFC3339)
		lastOpenedAt = &value
	}
	lastPageNumber := doc.LastPageNumber
	if lastPageNumber <= 0 {
		lastPageNumber = 1
	}
	return GetDocumentResponse{
		ID:             doc.ID,
		Filename:       doc.Filename,
		PageCount:      doc.PageCount,
		LastPageNumber: lastPageNumber,
		LastOpenedAt:   lastOpenedAt,
		CreatedAt:      doc.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      doc.UpdatedAt.Format(time.RFC3339),
	}
}

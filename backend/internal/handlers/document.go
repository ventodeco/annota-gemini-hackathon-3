package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
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

const (
	unreadablePDFMessage   = "This file couldn't be read as a PDF. Try another file."
	pageTextFailureMessage = "Couldn't read the text on this page. Please try again."
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

	pdfData, header, err := readFormFile(w, r, h.config.MaxUploadSize, "file")
	if err != nil {
		switch {
		case errors.Is(err, ErrUploadTooLarge):
			log.Warnf("PDF upload exceeds max size %d", h.config.MaxUploadSize)
			httputil.WriteJSONError(w, http.StatusRequestEntityTooLarge, fileTooLargeMBMessage(h.config.MaxUploadSize))
		case errors.Is(err, ErrMultipartParse):
			// Parser errors quote the malformed request; log only the classification.
			log.Warn("Failed to parse multipart form")
			httputil.WriteJSONError(w, http.StatusBadRequest, "Failed to parse form")
		case errors.Is(err, ErrMultipartField):
			log.Warn("Missing file field")
			httputil.WriteJSONError(w, http.StatusBadRequest, "Please select a PDF file to upload")
		default:
			log.Error("Failed to read uploaded file")
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

	var pageCount int
	stored := runWithAccountLock(w, r, h.db, log, userID, false, "Failed to save PDF file", func(ctx context.Context) error {
		docID, err := h.db.CreateDocument(ctx, doc)
		if err != nil {
			return &lockedStepFailure{http.StatusInternalServerError, "Failed to create document record"}
		}
		doc.ID = docID

		filePath, err := h.fileStorage.SavePDF(strconv.FormatInt(docID, 10), pdfData)
		if err != nil {
			log.ErrorWithErr(err, "Failed to save PDF file")
			h.discardFailedUpload(ctx, log, doc)
			return &lockedStepFailure{http.StatusInternalServerError, "Failed to save PDF file"}
		}

		pageCount, err = h.pdfExtractor.PageCount(filePath)
		if err != nil || pageCount < 1 {
			// Parser errors can quote bytes of the uploaded file; log none of it.
			log.WithField("document_id", docID).Warn("Uploaded PDF could not be parsed")
			h.discardFailedUpload(ctx, log, doc)
			return &lockedStepFailure{http.StatusUnprocessableEntity, unreadablePDFMessage}
		}

		if err := h.db.UpdateDocumentFileInfo(ctx, docID, filePath, pageCount); err != nil {
			log.ErrorWithErr(err, "Failed to update document record")
			h.discardFailedUpload(ctx, log, doc)
			return &lockedStepFailure{http.StatusInternalServerError, "Failed to update document record"}
		}
		return nil
	})
	if !stored {
		return
	}
	docID := doc.ID

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

	case len(parts) == 2 && parts[1] == "open":
		if r.Method != http.MethodPost {
			httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		h.recordDocumentOpenedHandler(w, r, doc)

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

	log := logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context())).WithUserID(userID)

	// Fast path for a page that already has its scan; creation below is
	// idempotent too, so this only skips re-extracting the text.
	existing, err := h.db.GetScanByDocumentPage(r.Context(), doc.ID, pageNumber)
	if err != nil {
		log.ErrorWithErr(err, "Failed to look up page scan")
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to prepare this page. Please try again.")
		return
	}
	if existing != nil {
		httputil.WriteJSON(w, http.StatusOK, CreateScanFromPageResponse{ScanID: existing.ID})
		return
	}

	// Extract text from page
	text, err := h.pdfExtractor.ExtractText(doc.FileURL, pageNumber)
	if err != nil {
		log.WithField("document_id", doc.ID).Warn("Failed to extract page text")
		httputil.WriteJSONError(w, http.StatusInternalServerError, pageTextFailureMessage)
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

	scanID, created, err := h.db.GetOrCreateDocumentPageScan(r.Context(), scan)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.WriteJSONError(w, http.StatusNotFound, "Document not found")
		return
	}
	if err != nil {
		log.ErrorWithErr(err, "Failed to create page scan")
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to prepare this page. Please try again.")
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	httputil.WriteJSON(w, status, CreateScanFromPageResponse{ScanID: scanID})
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

// recordDocumentOpenedHandler stamps the open time only. Progress is left to
// PATCH /progress so an open from a stale page can't overwrite newer progress.
func (h *DocumentHandlers) recordDocumentOpenedHandler(w http.ResponseWriter, r *http.Request, doc *models.Document) {
	userID := middleware.GetUserID(r.Context())
	if err := h.db.RecordDocumentOpened(r.Context(), doc.ID, userID); err != nil {
		httputil.WriteJSONError(w, http.StatusNotFound, "Document not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// deleteDocumentHandler removes the PDF first and stops on failure so a retry
// can finish: once the row is gone nothing points at the file. The document
// and its page scans are then removed in one transaction, so no page scan
// (and its extracted text) outlives the document. Both run under the
// exclusive account lock so an in-flight upload can't write its file after
// its row is gone. Saved annotations are kept; they are separate learning items.
func (h *DocumentHandlers) deleteDocumentHandler(w http.ResponseWriter, r *http.Request, doc *models.Document) {
	userID := middleware.GetUserID(r.Context())
	log := logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context())).WithUserID(userID)
	deleted := runWithAccountLock(w, r, h.db, log, userID, true, "Failed to delete document", func(ctx context.Context) error {
		if err := h.fileStorage.DeletePDF(documentPDFPath(h.config.UploadDir, doc)); err != nil {
			log.ErrorWithErr(err, "Failed to delete PDF file")
			return &lockedStepFailure{http.StatusInternalServerError, "Failed to delete document file"}
		}
		if err := h.db.DeleteDocumentAndPageScans(ctx, doc.ID, userID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return &lockedStepFailure{http.StatusNotFound, "Document not found"}
			}
			log.ErrorWithErr(err, "Failed to delete document records")
			return &lockedStepFailure{http.StatusInternalServerError, "Failed to delete document"}
		}
		return nil
	})
	if deleted {
		w.WriteHeader(http.StatusNoContent)
	}
}

// discardFailedUpload removes the PDF and record of an upload that could not
// be completed (for example a malformed PDF), so neither is left behind.
func (h *DocumentHandlers) discardFailedUpload(ctx context.Context, log *logger.Logger, doc *models.Document) {
	if err := h.fileStorage.DeletePDF(documentPDFPath(h.config.UploadDir, doc)); err != nil {
		log.ErrorWithErr(err, "Failed to remove PDF of failed upload")
		return
	}
	if err := h.db.DeleteDocument(ctx, doc.ID, doc.UserID); err != nil {
		log.ErrorWithErr(err, "Failed to remove record of failed upload")
	}
}

// documentPDFPath is the stored PDF location. Uploads that failed before the
// record was updated have no file_url, but their file may still exist at the
// path SavePDF derives from the document ID.
func documentPDFPath(uploadDir string, doc *models.Document) string {
	if doc.FileURL != "" {
		return doc.FileURL
	}
	return filepath.Join(uploadDir, "documents", strconv.FormatInt(doc.ID, 10)+".pdf")
}

func (h *DocumentHandlers) getPageTextHandler(w http.ResponseWriter, doc *models.Document, pageNumber int) {
	if pageNumber < 1 || pageNumber > doc.PageCount {
		httputil.WriteJSONError(w, http.StatusNotFound, "Page not found")
		return
	}

	text, err := h.pdfExtractor.ExtractText(doc.FileURL, pageNumber)
	if err != nil {
		httputil.WriteJSONError(w, http.StatusInternalServerError, pageTextFailureMessage)
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

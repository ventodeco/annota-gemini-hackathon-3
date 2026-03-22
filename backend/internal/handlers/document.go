package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
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

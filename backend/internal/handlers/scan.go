package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gemini-hackathon/app/internal/ai"
	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/httputil"
	"github.com/gemini-hackathon/app/internal/logger"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/services"
	"github.com/gemini-hackathon/app/internal/storage"
	"github.com/gemini-hackathon/app/internal/usage"
)

type ScanHandlers struct {
	scans  *services.ScanService
	config *config.Config
}

// OCRReservationStaleAfter is when an unsettled OCR reservation is refunded;
// see services.OCRReservationStaleAfter.
const OCRReservationStaleAfter = services.OCRReservationStaleAfter

func NewScanHandlers(db storage.DB, fileStorage storage.FileStorage, aiClient ai.Client, cfg *config.Config, meter *usage.Meter) *ScanHandlers {
	return &ScanHandlers{
		scans:  services.NewScanService(db, fileStorage, aiClient, meter, cfg.UploadDir),
		config: cfg,
	}
}

type CreateScanResponse struct {
	ScanID        int64   `json:"scanId"`
	FullText      string  `json:"fullText,omitempty"`
	ImageURL      string  `json:"imageUrl"`
	SourceType    string  `json:"sourceType"`
	Status        string  `json:"status"`
	FailureReason *string `json:"failureReason,omitempty"`
}

type ScanListItem struct {
	ID               int64   `json:"id"`
	ImageURL         string  `json:"imageUrl"`
	DetectedLanguage *string `json:"detectedLanguage,omitempty"`
	SourceType       string  `json:"sourceType"`
	Status           string  `json:"status"`
	FailureReason    *string `json:"failureReason,omitempty"`
	DocumentID       *int64  `json:"documentId,omitempty"`
	PageNumber       *int    `json:"pageNumber,omitempty"`
	CreatedAt        string  `json:"createdAt"`
}

type GetScansResponse struct {
	Data []ScanListItem `json:"data"`
	Meta PaginationMeta `json:"meta"`
}

type PaginationMeta struct {
	CurrentPage  int  `json:"currentPage"`
	PageSize     int  `json:"pageSize"`
	NextPage     *int `json:"nextPage,omitempty"`
	PreviousPage *int `json:"previousPage,omitempty"`
}

type GetScanResponse struct {
	ID               int64   `json:"id"`
	FullText         string  `json:"fullText,omitempty"`
	ImageURL         string  `json:"imageUrl"`
	DetectedLanguage *string `json:"detectedLanguage,omitempty"`
	SourceType       string  `json:"sourceType"`
	Status           string  `json:"status"`
	FailureReason    *string `json:"failureReason,omitempty"`
	DocumentID       *int64  `json:"documentId,omitempty"`
	PageNumber       *int    `json:"pageNumber,omitempty"`
	CreatedAt        string  `json:"createdAt"`
}

func (h *ScanHandlers) CreateScanAPI(w http.ResponseWriter, r *http.Request) {
	log := logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context()))

	if r.Method != http.MethodPost {
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	log = log.WithUserID(userID)

	imageData, header, err := readFormFile(w, r, h.config.MaxUploadSize, "image")
	if err != nil {
		switch {
		case errors.Is(err, ErrUploadTooLarge):
			log.Warnf("Image upload exceeds max size %d", h.config.MaxUploadSize)
			httputil.WriteJSONError(w, http.StatusRequestEntityTooLarge, fileTooLargeMBMessage(h.config.MaxUploadSize))
		case errors.Is(err, ErrMultipartParse):
			// Parser errors quote the malformed request; log only the classification.
			log.Warn("Failed to parse multipart form")
			httputil.WriteJSONError(w, http.StatusBadRequest, "Failed to parse form")
		case errors.Is(err, ErrMultipartField):
			log.Warn("Missing image field")
			httputil.WriteJSONError(w, http.StatusBadRequest, "Please select an image to upload")
		default:
			log.Error("Failed to read uploaded file")
			httputil.WriteJSONError(w, http.StatusBadRequest, "Failed to read uploaded file")
		}
		return
	}

	mimeType := header.Header.Get("Content-Type")
	if !isValidImageType(mimeType) {
		log.Warnf("Invalid image type received: %s", mimeType)
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid image type. Please use JPEG, PNG, or WebP.")
		return
	}

	if header.Size > h.config.MaxUploadSize {
		log.Warnf("Image size %d exceeds max size %d", header.Size, h.config.MaxUploadSize)
		httputil.WriteJSONError(w, http.StatusRequestEntityTooLarge, fileTooLargeMBMessage(h.config.MaxUploadSize))
		return
	}

	log.Infof("Received image upload: size=%d bytes, type=%s", len(imageData), mimeType)

	created, err := h.scans.CreateImageScan(r.Context(), userID, imageData, mimeType)
	if err != nil {
		var admission *services.AdmissionError
		switch {
		case errors.As(err, &admission):
			writeUsageError(w, admission.Err, log)
		case errors.Is(err, services.ErrScanCreate):
			httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to initialize scan")
		default:
			httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to save uploaded image")
		}
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, CreateScanResponse{
		ScanID:        created.ID,
		ImageURL:      created.ImageURL,
		SourceType:    "image",
		Status:        created.Status,
		FailureReason: created.FailureReason,
	})
}

func (h *ScanHandlers) GetScansAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	log := logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context())).WithUserID(userID)

	page, size := httputil.ParsePagination(r, h.config.DefaultPageSize)

	scans, err := h.scans.ListScans(r.Context(), userID, page, size)
	if err != nil {
		log.ErrorWithErr(err, "Failed to get scans from database")
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to get scans")
		return
	}

	log.Infof("Retrieved %d scans for user (page=%d, size=%d)", len(scans), page, size)

	data := make([]ScanListItem, len(scans))
	for i, scan := range scans {
		data[i] = ScanListItem{
			ID:               scan.ID,
			ImageURL:         scan.ImageURL,
			DetectedLanguage: scan.DetectedLanguage,
			SourceType:       services.SourceTypeOrDefault(scan),
			Status:           services.StatusOrDefault(scan),
			FailureReason:    scan.FailureReason,
			DocumentID:       scan.DocumentID,
			PageNumber:       scan.PageNumber,
			CreatedAt:        scan.CreatedAt.Format(time.RFC3339),
		}
	}

	var nextPage, prevPage *int
	if len(scans) == size {
		nextPageVal := page + 1
		nextPage = &nextPageVal
	}
	if page > 1 {
		prevPageVal := page - 1
		prevPage = &prevPageVal
	}

	response := GetScansResponse{
		Data: data,
		Meta: PaginationMeta{
			CurrentPage:  page,
			PageSize:     size,
			NextPage:     nextPage,
			PreviousPage: prevPage,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *ScanHandlers) GetScanAPI(w http.ResponseWriter, r *http.Request) {
	h.ScanByIDAPI(w, r)
}

func (h *ScanHandlers) ScanByIDAPI(w http.ResponseWriter, r *http.Request) {
	log := logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context()))

	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/scans/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) == 2 && parts[1] == "image" {
		scanID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || scanID <= 0 {
			httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid scan ID")
			return
		}
		if r.Method != http.MethodGet {
			httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		h.getScanImageHandler(w, r, scanID, log)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getScanHandler(w, r, log)
	case http.MethodDelete:
		h.deleteScanHandler(w, r, log)
	default:
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *ScanHandlers) deleteScanHandler(w http.ResponseWriter, r *http.Request, log *logger.Logger) {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	log = log.WithUserID(userID)

	path := strings.TrimPrefix(r.URL.Path, "/v1/scans/")
	scanIDStr := strings.TrimSuffix(path, "/")
	scanID, err := strconv.ParseInt(scanIDStr, 10, 64)
	if err != nil || scanID <= 0 {
		log.Warnf("Invalid scan ID format: %s", scanIDStr)
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid scan ID")
		return
	}

	log = log.WithField("scan_id", scanID)

	if err := h.scans.DeleteScan(r.Context(), userID, scanID); err != nil {
		switch {
		case errors.Is(err, services.ErrScanNotFound):
			log.ErrorWithErr(err, "Failed to get scan by ID")
			httputil.WriteJSONError(w, http.StatusNotFound, "Scan not found")
		case errors.Is(err, services.ErrScanForbidden):
			log.Warn("User attempted to delete scan belonging to another user")
			httputil.WriteJSONError(w, http.StatusForbidden, "Access denied")
		default:
			httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to delete scan")
		}
		return
	}

	log.Infof("Scan deleted successfully: id=%d", scanID)
	w.WriteHeader(http.StatusNoContent)
}

func (h *ScanHandlers) getScanHandler(w http.ResponseWriter, r *http.Request, log *logger.Logger) {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	log = log.WithUserID(userID)

	path := strings.TrimPrefix(r.URL.Path, "/v1/scans/")
	scanIDStr := strings.TrimSuffix(path, "/")
	scanID, err := strconv.ParseInt(scanIDStr, 10, 64)
	if err != nil {
		log.Warnf("Invalid scan ID format: %s", scanIDStr)
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid scan ID")
		return
	}

	log = log.WithField("scan_id", scanID)

	scan, err := h.scans.GetScan(r.Context(), userID, scanID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrScanForbidden):
			log.Warn("User attempted to access scan belonging to another user")
			httputil.WriteJSONError(w, http.StatusForbidden, "Access denied")
		default:
			log.ErrorWithErr(err, "Failed to get scan")
			httputil.WriteJSONError(w, http.StatusNotFound, "Scan not found")
		}
		return
	}

	fullText := ""
	if scan.FullOCRText != nil {
		fullText = *scan.FullOCRText
	}

	log.WithFields(map[string]any{
		"has_ocr":         fullText != "",
		"ocr_text_length": len(fullText),
		"language":        scan.DetectedLanguage,
	}).Infof("Successfully retrieved scan: id=%d", scanID)

	response := GetScanResponse{
		ID:               scan.ID,
		FullText:         fullText,
		ImageURL:         scan.ImageURL,
		DetectedLanguage: scan.DetectedLanguage,
		SourceType:       services.SourceTypeOrDefault(scan),
		Status:           services.StatusOrDefault(scan),
		FailureReason:    scan.FailureReason,
		DocumentID:       scan.DocumentID,
		PageNumber:       scan.PageNumber,
		CreatedAt:        scan.CreatedAt.Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *ScanHandlers) getScanImageHandler(w http.ResponseWriter, r *http.Request, scanID int64, log *logger.Logger) {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	data, contentType, err := h.scans.ScanImage(r.Context(), userID, scanID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrScanForbidden):
			log.WithUserID(userID).Warn("User attempted to access scan image belonging to another user")
			httputil.WriteJSONError(w, http.StatusForbidden, "Access denied")
		case errors.Is(err, services.ErrScanImageNotFound):
			log.WithUserID(userID).ErrorWithErr(err, "Failed to read scan image")
			httputil.WriteJSONError(w, http.StatusNotFound, "Scan image not found")
		default:
			httputil.WriteJSONError(w, http.StatusNotFound, "Scan not found")
		}
		return
	}

	w.Header().Set("Content-Type", contentType)
	// Images are fetched with a bearer token but cached by URL only; storing
	// them would let another account on this browser, or a deleted scan, be shown.
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		log.WithUserID(userID).ErrorWithErr(err, "Failed to write scan image response")
	}
}

func isValidImageType(mimeType string) bool {
	validTypes := []string{"image/jpeg", "image/jpg", "image/png", "image/webp"}
	for _, valid := range validTypes {
		if mimeType == valid {
			return true
		}
	}
	return false
}

func (h *ScanHandlers) ScansAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.CreateScanAPI(w, r)
	case http.MethodGet:
		h.GetScansAPI(w, r)
	default:
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gemini-hackathon/app/internal/ai"
	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/httputil"
	"github.com/gemini-hackathon/app/internal/logger"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/storage"
)

type ScanHandlers struct {
	db          storage.DB
	fileStorage storage.FileStorage
	aiClient    ai.Client
	config      *config.Config
}

const ocrProcessingTimeout = 5 * time.Minute

func NewScanHandlers(db storage.DB, fileStorage storage.FileStorage, aiClient ai.Client, cfg *config.Config) *ScanHandlers {
	return &ScanHandlers{
		db:          db,
		fileStorage: fileStorage,
		aiClient:    aiClient,
		config:      cfg,
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
			log.Warnf("Image upload exceeds max size %d: %v", h.config.MaxUploadSize, err)
			httputil.WriteJSONError(w, http.StatusRequestEntityTooLarge, fileTooLargeMBMessage(h.config.MaxUploadSize))
		case errors.Is(err, ErrMultipartParse):
			log.Warnf("Failed to parse multipart form: %v", err)
			httputil.WriteJSONError(w, http.StatusBadRequest, "Failed to parse form")
		case errors.Is(err, ErrMultipartField):
			log.Warnf("Failed to get image from form: %v", err)
			httputil.WriteJSONError(w, http.StatusBadRequest, "Please select an image to upload")
		default:
			log.ErrorWithErr(err, "Failed to read uploaded file")
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

	now := time.Now()
	scan := &models.Scan{
		UserID:     userID,
		ImageURL:   "",
		SourceType: "image",
		Status:     "processing",
		CreatedAt:  now,
	}

	scanID, err := h.db.CreateScan(r.Context(), scan)
	if err != nil {
		log.ErrorWithErr(err, "Failed to create scan in database")
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to initialize scan")
		return
	}

	storagePath, _, err := h.fileStorage.SaveImage(strconv.FormatInt(scanID, 10), imageData, mimeType)
	if err != nil {
		log.ErrorWithErr(err, "Failed to save image to storage")
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to save uploaded image")
		return
	}

	imageURL := privateScanImageURL(scanID)

	if err := h.db.UpdateScanImageURL(r.Context(), scanID, imageURL); err != nil {
		log.ErrorWithErr(err, "Failed to update scan with image URL")
	}

	log.WithFields(map[string]any{
		"scan_id":   scanID,
		"user_id":   userID,
		"image_url": imageURL,
	}).Infof("Scan created successfully, starting OCR processing")

	ocrCtx, cancelOCR := context.WithTimeout(context.WithoutCancel(r.Context()), ocrProcessingTimeout)
	go func() {
		defer cancelOCR()
		h.processOCR(ocrCtx, scanID, imageData, mimeType, storagePath)
	}()

	response := CreateScanResponse{
		ScanID:     scanID,
		FullText:   "",
		ImageURL:   imageURL,
		SourceType: "image",
		Status:     "processing",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
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

	scans, err := h.db.GetScansByUserID(r.Context(), userID, page, size)
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
			SourceType:       sourceTypeOrDefault(scan),
			Status:           statusOrDefault(scan),
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

	scan, err := h.db.GetScanByID(r.Context(), scanID)
	if err != nil {
		log.ErrorWithErr(err, "Failed to get scan by ID")
		httputil.WriteJSONError(w, http.StatusNotFound, "Scan not found")
		return
	}
	if scan == nil {
		httputil.WriteJSONError(w, http.StatusNotFound, "Scan not found")
		return
	}
	if scan.UserID != userID {
		log.Warn("User attempted to delete scan belonging to another user")
		httputil.WriteJSONError(w, http.StatusForbidden, "Access denied")
		return
	}

	if scan.ImageURL != "" {
		if err := h.deleteStoredScanImage(scan); err != nil {
			log.Warnf("Failed to delete image file (continuing with DB delete): %v", err)
		}
	}

	if err := h.db.DeleteScan(r.Context(), scanID, userID); err != nil {
		log.ErrorWithErr(err, "Failed to delete scan")
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to delete scan")
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

	scan, err := h.db.GetScanByID(r.Context(), scanID)
	if err != nil {
		log.ErrorWithErr(err, "Failed to get scan by ID from database")
		httputil.WriteJSONError(w, http.StatusNotFound, "Scan not found")
		return
	}

	if scan == nil {
		log.Warn("Scan not found")
		httputil.WriteJSONError(w, http.StatusNotFound, "Scan not found")
		return
	}

	if scan.UserID != userID {
		log.Warn("User attempted to access scan belonging to another user")
		httputil.WriteJSONError(w, http.StatusForbidden, "Access denied")
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
		SourceType:       sourceTypeOrDefault(scan),
		Status:           statusOrDefault(scan),
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

	scan, err := h.db.GetScanByID(r.Context(), scanID)
	if err != nil || scan == nil {
		httputil.WriteJSONError(w, http.StatusNotFound, "Scan not found")
		return
	}
	if scan.UserID != userID {
		log.WithUserID(userID).Warn("User attempted to access scan image belonging to another user")
		httputil.WriteJSONError(w, http.StatusForbidden, "Access denied")
		return
	}

	data, contentType, err := h.openStoredScanImage(scan)
	if err != nil {
		log.WithUserID(userID).ErrorWithErr(err, "Failed to read scan image")
		httputil.WriteJSONError(w, http.StatusNotFound, "Scan image not found")
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		log.WithUserID(userID).ErrorWithErr(err, "Failed to write scan image response")
	}
}

func (h *ScanHandlers) processOCR(ctx context.Context, scanID int64, imageData []byte, mimeType string, storagePath string) {
	log := logger.GetDefaultLogger().WithField("scan_id", scanID)

	log.Infof("Starting OCR processing: image_size=%d bytes, mime_type=%s", len(imageData), mimeType)
	ocrResp, err := h.aiClient.OCR(ctx, imageData, mimeType)
	if err != nil {
		log.ErrorWithErr(err, "OCR processing failed")
		reason := "ocr_failed"
		if err := h.db.UpdateScanStatus(ctx, scanID, "failed", &reason); err != nil {
			log.ErrorWithErr(err, "Failed to persist OCR failure")
		}
		return
	}
	log.Infof("OCR completed successfully: language=%s, text_length=%d", ocrResp.Language, len(ocrResp.RawText))

	imageURL := fmt.Sprintf("/uploads/%s", storagePath)

	_ = imageURL

	if err := h.db.UpdateScanOCR(ctx, scanID, ocrResp.RawText, ocrResp.Language); err != nil {
		log.ErrorWithErr(err, "Failed to update scan OCR in database")
		return
	}

	log.Infof("OCR results saved to database successfully")
}

func (h *ScanHandlers) openStoredScanImage(scan *models.Scan) ([]byte, string, error) {
	for _, candidate := range scanImagePathCandidates(h.config.UploadDir, scan) {
		data, err := h.fileStorage.OpenImage(candidate.path)
		if err == nil {
			return data, candidate.contentType, nil
		}
	}
	return nil, "", fmt.Errorf("scan image not found")
}

func (h *ScanHandlers) deleteStoredScanImage(scan *models.Scan) error {
	var lastErr error
	for _, candidate := range scanImagePathCandidates(h.config.UploadDir, scan) {
		if err := h.fileStorage.DeleteImage(candidate.path); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

type scanImagePathCandidate struct {
	path        string
	contentType string
}

func scanImagePathCandidates(uploadDir string, scan *models.Scan) []scanImagePathCandidate {
	if scan.ImageURL != "" && strings.HasPrefix(scan.ImageURL, "/uploads/") {
		path := filepath.Join(uploadDir, filepath.Base(scan.ImageURL))
		return []scanImagePathCandidate{{path: path, contentType: contentTypeFromImagePath(path)}}
	}

	base := filepath.Join(uploadDir, strconv.FormatInt(scan.ID, 10))
	return []scanImagePathCandidate{
		{path: base + ".jpg", contentType: "image/jpeg"},
		{path: base + ".png", contentType: "image/png"},
		{path: base + ".webp", contentType: "image/webp"},
	}
}

func privateScanImageURL(scanID int64) string {
	return fmt.Sprintf("/v1/scans/%d/image", scanID)
}

func contentTypeFromImagePath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
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

func sourceTypeOrDefault(scan *models.Scan) string {
	if scan.SourceType != "" {
		return scan.SourceType
	}
	if scan.DocumentID != nil {
		return "pdf"
	}
	return "image"
}

func statusOrDefault(scan *models.Scan) string {
	if scan.Status != "" {
		return scan.Status
	}
	if scan.FullOCRText != nil && strings.TrimSpace(*scan.FullOCRText) != "" {
		return "ready"
	}
	return "processing"
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

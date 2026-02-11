package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/gemini"
	"github.com/gemini-hackathon/app/internal/logger"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/storage"
)

type ScanHandlers struct {
	db           storage.DB
	fileStorage  storage.FileStorage
	geminiClient gemini.Client
	config       *config.Config
}

func NewScanHandlers(db storage.DB, fileStorage storage.FileStorage, geminiClient gemini.Client, cfg *config.Config) *ScanHandlers {
	return &ScanHandlers{
		db:           db,
		fileStorage:  fileStorage,
		geminiClient: geminiClient,
		config:       cfg,
	}
}

type CreateScanResponse struct {
	ScanID   int64  `json:"scanId"`
	FullText string `json:"fullText,omitempty"`
	ImageURL string `json:"imageUrl"`
}

type ScanListItem struct {
	ID               int64   `json:"id"`
	ImageURL         string  `json:"imageUrl"`
	DetectedLanguage *string `json:"detectedLanguage,omitempty"`
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
	CreatedAt        string  `json:"createdAt"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func (h *ScanHandlers) CreateScanAPI(w http.ResponseWriter, r *http.Request) {
	log := logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context()))

	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	log = log.WithUserID(userID)

	if err := r.ParseMultipartForm(h.config.MaxUploadSize); err != nil {
		log.Warnf("Failed to parse multipart form: %v", err)
		h.writeJSONError(w, http.StatusBadRequest, "Failed to parse form")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		log.Warnf("Failed to get image from form: %v", err)
		h.writeJSONError(w, http.StatusBadRequest, "Please select an image to upload")
		return
	}
	defer file.Close()

	mimeType := header.Header.Get("Content-Type")
	if !isValidImageType(mimeType) {
		log.Warnf("Invalid image type received: %s", mimeType)
		h.writeJSONError(w, http.StatusBadRequest, "Invalid image type. Please use JPEG, PNG, or WebP.")
		return
	}

	if header.Size > h.config.MaxUploadSize {
		log.Warnf("Image size %d exceeds max size %d", header.Size, h.config.MaxUploadSize)
		h.writeJSONError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("File too large. Maximum size is %v MB.", h.config.MaxUploadSize/(1024*1024)))
		return
	}

	imageData, err := io.ReadAll(file)
	if err != nil {
		log.ErrorWithErr(err, "Failed to read uploaded file")
		h.writeJSONError(w, http.StatusBadRequest, "Failed to read uploaded file")
		return
	}

	log.Infof("Received image upload: size=%d bytes, type=%s", len(imageData), mimeType)

	now := time.Now()
	scan := &models.Scan{
		UserID:    userID,
		ImageURL:  "",
		CreatedAt: now,
	}

	scanID, err := h.db.CreateScan(r.Context(), scan)
	if err != nil {
		log.ErrorWithErr(err, "Failed to create scan in database")
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to initialize scan")
		return
	}

	storagePath, _, err := h.fileStorage.SaveImage(strconv.FormatInt(scanID, 10), imageData, mimeType)
	if err != nil {
		log.ErrorWithErr(err, "Failed to save image to storage")
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to save uploaded image")
		return
	}

	imageURL := fmt.Sprintf("/uploads/%s", filepath.Base(storagePath))

	if err := h.db.UpdateScanImageURL(r.Context(), scanID, imageURL); err != nil {
		log.ErrorWithErr(err, "Failed to update scan with image URL")
	}

	log.WithFields(map[string]any{
		"scan_id":   scanID,
		"user_id":   userID,
		"image_url": imageURL,
	}).Infof("Scan created successfully, starting OCR processing")

	go h.processOCR(context.Background(), scanID, imageData, mimeType, storagePath)

	response := CreateScanResponse{
		ScanID:   scanID,
		FullText: "",
		ImageURL: imageURL,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func (h *ScanHandlers) GetScansAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	log := logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context())).WithUserID(userID)

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size < 1 {
		size = h.config.DefaultPageSize
	}
	if size > 100 {
		size = 100
	}

	scans, err := h.db.GetScansByUserID(r.Context(), userID, page, size)
	if err != nil {
		log.ErrorWithErr(err, "Failed to get scans from database")
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to get scans")
		return
	}

	log.Infof("Retrieved %d scans for user (page=%d, size=%d)", len(scans), page, size)

	data := make([]ScanListItem, len(scans))
	for i, scan := range scans {
		data[i] = ScanListItem{
			ID:               scan.ID,
			ImageURL:         scan.ImageURL,
			DetectedLanguage: scan.DetectedLanguage,
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

	switch r.Method {
	case http.MethodGet:
		h.getScanHandler(w, r, log)
	case http.MethodDelete:
		h.deleteScanHandler(w, r, log)
	default:
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *ScanHandlers) deleteScanHandler(w http.ResponseWriter, r *http.Request, log *logger.Logger) {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	log = log.WithUserID(userID)

	path := strings.TrimPrefix(r.URL.Path, "/v1/scans/")
	scanIDStr := strings.TrimSuffix(path, "/")
	scanID, err := strconv.ParseInt(scanIDStr, 10, 64)
	if err != nil || scanID <= 0 {
		log.Warnf("Invalid scan ID format: %s", scanIDStr)
		h.writeJSONError(w, http.StatusBadRequest, "Invalid scan ID")
		return
	}

	log = log.WithField("scan_id", scanID)

	scan, err := h.db.GetScanByID(r.Context(), scanID)
	if err != nil {
		log.ErrorWithErr(err, "Failed to get scan by ID")
		h.writeJSONError(w, http.StatusNotFound, "Scan not found")
		return
	}
	if scan == nil {
		h.writeJSONError(w, http.StatusNotFound, "Scan not found")
		return
	}
	if scan.UserID != userID {
		log.Warn("User attempted to delete scan belonging to another user")
		h.writeJSONError(w, http.StatusForbidden, "Access denied")
		return
	}

	imagePath := filepath.Join(h.config.UploadDir, filepath.Base(scan.ImageURL))
	if err := h.fileStorage.DeleteImage(imagePath); err != nil {
		log.Warnf("Failed to delete image file (continuing with DB delete): %v", err)
	}

	if err := h.db.DeleteScan(r.Context(), scanID, userID); err != nil {
		log.ErrorWithErr(err, "Failed to delete scan")
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to delete scan")
		return
	}

	log.Infof("Scan deleted successfully: id=%d", scanID)
	w.WriteHeader(http.StatusNoContent)
}

func (h *ScanHandlers) getScanHandler(w http.ResponseWriter, r *http.Request, log *logger.Logger) {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	log = log.WithUserID(userID)

	path := strings.TrimPrefix(r.URL.Path, "/v1/scans/")
	scanIDStr := strings.TrimSuffix(path, "/")
	scanID, err := strconv.ParseInt(scanIDStr, 10, 64)
	if err != nil {
		log.Warnf("Invalid scan ID format: %s", scanIDStr)
		h.writeJSONError(w, http.StatusBadRequest, "Invalid scan ID")
		return
	}

	log = log.WithField("scan_id", scanID)

	scan, err := h.db.GetScanByID(r.Context(), scanID)
	if err != nil {
		log.ErrorWithErr(err, "Failed to get scan by ID from database")
		h.writeJSONError(w, http.StatusNotFound, "Scan not found")
		return
	}

	if scan == nil {
		log.Warn("Scan not found")
		h.writeJSONError(w, http.StatusNotFound, "Scan not found")
		return
	}

	if scan.UserID != userID {
		log.Warn("User attempted to access scan belonging to another user")
		h.writeJSONError(w, http.StatusForbidden, "Access denied")
		return
	}

	fullText := ""
	if scan.FullOCRText != nil {
		fullText = *scan.FullOCRText
	}

	log.WithFields(map[string]any{
		"has_ocr":          fullText != "",
		"ocr_text_length":  len(fullText),
		"language":         scan.DetectedLanguage,
	}).Infof("Successfully retrieved scan: id=%d", scanID)

	response := GetScanResponse{
		ID:               scan.ID,
		FullText:         fullText,
		ImageURL:         scan.ImageURL,
		DetectedLanguage: scan.DetectedLanguage,
		CreatedAt:        scan.CreatedAt.Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *ScanHandlers) processOCR(ctx context.Context, scanID int64, imageData []byte, mimeType string, storagePath string) {
	log := logger.GetDefaultLogger().WithField("scan_id", scanID)

	log.Infof("Starting OCR processing: image_size=%d bytes, mime_type=%s", len(imageData), mimeType)
	ocrResp, err := h.geminiClient.OCR(ctx, imageData, mimeType)
	if err != nil {
		log.ErrorWithErr(err, "OCR processing failed")
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

func (h *ScanHandlers) writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
	})
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

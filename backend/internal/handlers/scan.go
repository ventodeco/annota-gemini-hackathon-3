package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/gemini"
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
	h.setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if err := r.ParseMultipartForm(h.config.MaxUploadSize); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Failed to parse form")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Please select an image to upload")
		return
	}
	defer file.Close()

	mimeType := header.Header.Get("Content-Type")
	if !isValidImageType(mimeType) {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid image type. Please use JPEG, PNG, or WebP.")
		return
	}

	if header.Size > h.config.MaxUploadSize {
		h.writeJSONError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("File too large. Maximum size is %v MB.", h.config.MaxUploadSize/(1024*1024)))
		return
	}

	imageData, err := io.ReadAll(file)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Failed to read uploaded file")
		return
	}

	now := time.Now()
	scan := &models.Scan{
		UserID:    userID,
		ImageURL:  "",
		CreatedAt: now,
	}

	scanID, err := h.db.CreateScan(r.Context(), scan)
	if err != nil {
		log.Printf("Failed to create scan: %v", err)
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to initialize scan")
		return
	}

	storagePath, _, err := h.fileStorage.SaveImage(strconv.FormatInt(scanID, 10), imageData, mimeType)
	if err != nil {
		log.Printf("Failed to save image: %v", err)
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to save uploaded image")
		return
	}

	imageURL := fmt.Sprintf("/uploads/%s", storagePath)

	err = h.db.UpdateScanOCR(r.Context(), scanID, "", "")
	if err != nil {
		log.Printf("Failed to update scan with image URL: %v", err)
	}

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
	h.setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

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
		log.Printf("Failed to get scans: %v", err)
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to get scans")
		return
	}

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
	h.setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/scans/")
	scanIDStr := strings.TrimSuffix(path, "/")
	scanID, err := strconv.ParseInt(scanIDStr, 10, 64)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid scan ID")
		return
	}

	scan, err := h.db.GetScanByID(r.Context(), scanID)
	if err != nil {
		log.Printf("GetScanByID error for scan %d: %v", scanID, err)
		h.writeJSONError(w, http.StatusNotFound, "Scan not found")
		return
	}

	if scan == nil {
		h.writeJSONError(w, http.StatusNotFound, "Scan not found")
		return
	}

	if scan.UserID != userID {
		h.writeJSONError(w, http.StatusForbidden, "Access denied")
		return
	}

	fullText := ""
	if scan.FullOCRText != nil {
		fullText = *scan.FullOCRText
	}

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
	log.Printf("Starting OCR processing for scan %d", scanID)
	ocrResp, err := h.geminiClient.OCR(ctx, imageData, mimeType)
	if err != nil {
		log.Printf("OCR failed for scan %d: %v", scanID, err)
		return
	}
	log.Printf("OCR successful for scan %d", scanID)

	imageURL := fmt.Sprintf("/uploads/%s", storagePath)

	_ = imageURL

	if err := h.db.UpdateScanOCR(ctx, scanID, ocrResp.RawText, ocrResp.Language); err != nil {
		log.Printf("Failed to update scan OCR for scan %d: %v", scanID, err)
	}
	log.Printf("OCR completed successfully for scan %d", scanID)
}

func (h *ScanHandlers) setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, x-token")
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

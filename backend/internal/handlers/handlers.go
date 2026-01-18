package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/gemini"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/storage"
)

type Handlers struct {
	db           storage.DB
	fileStorage  storage.FileStorage
	geminiClient gemini.Client
	config       *config.Config
}

func NewHandlers(db storage.DB, fileStorage storage.FileStorage, geminiClient gemini.Client, cfg *config.Config) *Handlers {
	return &Handlers{
		db:           db,
		fileStorage:  fileStorage,
		geminiClient: geminiClient,
		config:       cfg,
	}
}

func (h *Handlers) Healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (h *Handlers) setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

type CreateScanAPIResponse struct {
	ScanID    string `json:"scanID"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

type GetScanAPIResponse struct {
	Scan      *models.Scan      `json:"scan"`
	OCRResult *models.OCRResult `json:"ocrResult"`
	Status    string            `json:"status"`
}

type AnnotateAPIRequest struct {
	SelectedText string `json:"selectedText"`
}

type ErrorAPIResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func (h *Handlers) CreateScanAPI(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	sessionID := middleware.GetSessionID(r.Context())
	if sessionID == "" {
		h.writeJSONError(w, http.StatusUnauthorized, "Session required")
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

	scanID := generateID()
	now := time.Now()

	scan := &models.Scan{
		ID:        scanID,
		SessionID: sessionID,
		Source:    "upload",
		Status:    "uploaded",
		CreatedAt: now,
	}

	if err := h.db.CreateScan(r.Context(), scan); err != nil {
		log.Printf("Failed to create scan: %v", err)
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to initialize scan")
		return
	}

	storagePath, sha256, err := h.fileStorage.SaveImage(scanID, imageData, mimeType)
	if err != nil {
		log.Printf("Failed to save image: %v", err)
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to save uploaded image")
		return
	}

	image := &models.ScanImage{
		ID:          generateID(),
		ScanID:      scanID,
		StoragePath: storagePath,
		MimeType:    mimeType,
		SHA256:      sha256,
		CreatedAt:   now,
	}

	if err := h.db.CreateScanImage(r.Context(), image); err != nil {
		log.Printf("Failed to create scan image: %v", err)
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to create scan image")
		return
	}

	go h.processOCR(context.Background(), scanID, imageData, mimeType)

	response := CreateScanAPIResponse{
		ScanID:    scanID,
		Status:    "uploaded",
		CreatedAt: now.Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func (h *Handlers) GetScanAPI(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/scans/")
	parts := strings.Split(path, "/")
	scanID := parts[0]
	if scanID == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Scan ID required")
		return
	}

	scan, ocrResult, err := h.db.GetScanWithOCR(r.Context(), scanID)
	if err != nil {
		log.Printf("GetScanWithOCR error for scan %s: %v", scanID, err)
		h.writeJSONError(w, http.StatusNotFound, "Scan not found")
		return
	}

	if scan == nil {
		h.writeJSONError(w, http.StatusNotFound, "Scan not found")
		return
	}

	response := GetScanAPIResponse{
		Scan:      scan,
		OCRResult: ocrResult,
		Status:    scan.Status,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handlers) AnnotateAPI(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	scanID := strings.TrimPrefix(r.URL.Path, "/api/scans/")
	scanID = strings.TrimSuffix(scanID, "/annotate")
	if scanID == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Scan ID required")
		return
	}

	var req AnnotateAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	selectedText := strings.TrimSpace(req.SelectedText)
	if selectedText == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Please select some text to annotate")
		return
	}

	if len(selectedText) > 1000 {
		h.writeJSONError(w, http.StatusBadRequest, "Selected text is too long")
		return
	}

	ocrResult, err := h.db.GetOCRResult(r.Context(), scanID)
	if err != nil {
		h.writeJSONError(w, http.StatusNotFound, "Failed to load OCR result")
		return
	}

	annotationResp, err := h.geminiClient.Annotate(r.Context(), ocrResult.RawText, selectedText)
	if err != nil {
		log.Printf("Failed to generate annotation: %v", err)
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to generate annotation. Please try again.")
		return
	}

	annotation := &models.Annotation{
		ID:                  generateID(),
		ScanID:              scanID,
		OCRResultID:         ocrResult.ID,
		SelectedText:        selectedText,
		Meaning:             annotationResp.Meaning,
		UsageExample:        annotationResp.UsageExample,
		WhenToUse:           annotationResp.WhenToUse,
		WordBreakdown:       annotationResp.WordBreakdown,
		AlternativeMeanings: annotationResp.AlternativeMeanings,
		Model:               "gemini-2.5-flash",
		PromptVersion:       "1.0",
		CreatedAt:           time.Now(),
	}

	if err := h.db.CreateAnnotation(r.Context(), annotation); err != nil {
		log.Printf("Failed to save annotation: %v", err)
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to save annotation")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(annotation)
}

func (h *Handlers) GetScanImage(w http.ResponseWriter, r *http.Request) {
	scanID := strings.TrimPrefix(r.URL.Path, "/api/scans/")
	scanID = strings.TrimSuffix(scanID, "/image")
	if scanID == "" {
		http.Error(w, "Scan ID required", http.StatusBadRequest)
		return
	}

	image, err := h.db.GetScanImage(r.Context(), scanID)
	if err != nil {
		http.Error(w, "Image not found", http.StatusNotFound)
		return
	}

	data, err := h.fileStorage.OpenImage(image.StoragePath)
	if err != nil {
		log.Printf("Failed to open image %s: %v", image.StoragePath, err)
		http.Error(w, "Failed to load image", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", image.MimeType)
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	w.Write(data)
}

func (h *Handlers) writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorAPIResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
	})
}

func (h *Handlers) processOCR(ctx context.Context, scanID string, imageData []byte, mimeType string) {
	log.Printf("Starting OCR processing for scan %s", scanID)
	ocrResp, err := h.geminiClient.OCR(ctx, imageData, mimeType)
	if err != nil {
		log.Printf("OCR failed for scan %s: %v", scanID, err)
		status := "failed"

		errMsgLower := strings.ToLower(err.Error())
		if strings.Contains(errMsgLower, "503") ||
			strings.Contains(errMsgLower, "429") ||
			strings.Contains(errMsgLower, "overloaded") ||
			strings.Contains(err.Error(), "UNAVAILABLE") ||
			strings.Contains(errMsgLower, "quota") {
			status = "failed_overloaded"
		} else if strings.Contains(err.Error(), "401") ||
			strings.Contains(err.Error(), "403") ||
			strings.Contains(errMsgLower, "api key") ||
			strings.Contains(err.Error(), "not initialized") {
			status = "failed_auth"
		}

		if updateErr := h.db.UpdateScanStatus(ctx, scanID, status); updateErr != nil {
			log.Printf("ERROR: Failed to update scan status to %s for scan %s: %v", status, scanID, updateErr)
		}
		return
	}
	log.Printf("OCR successful for scan %s, extracted %d characters", scanID, len(ocrResp.RawText))

	ocrResult := &models.OCRResult{
		ID:             generateID(),
		ScanID:         scanID,
		Model:          "gemini-2.5-flash",
		Language:       &ocrResp.Language,
		RawText:        ocrResp.RawText,
		StructuredJSON: &ocrResp.StructuredJSON,
		PromptVersion:  "1.0",
		CreatedAt:      time.Now(),
	}

	if err := h.db.CreateOCRResult(ctx, ocrResult); err != nil {
		log.Printf("Failed to save OCR result: %v", err)
		return
	}

	if err := h.db.UpdateScanStatus(ctx, scanID, "ocr_done"); err != nil {
		log.Printf("Failed to update scan status: %v", err)
		return
	}
	log.Printf("OCR completed successfully for scan %s", scanID)
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

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

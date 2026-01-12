package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/gemini"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/storage"
)

type Handlers struct {
	db          storage.DB
	fileStorage storage.FileStorage
	geminiClient gemini.Client
	config      *config.Config
	templates   *template.Template
}

func NewHandlers(db storage.DB, fileStorage storage.FileStorage, geminiClient gemini.Client, cfg *config.Config) (*Handlers, error) {
	tmpl, err := loadTemplates()
	if err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	return &Handlers{
		db:          db,
		fileStorage: fileStorage,
		geminiClient: geminiClient,
		config:      cfg,
		templates:   tmpl,
	}, nil
}

func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{}
	if err := h.templates.ExecuteTemplate(w, "home", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handlers) CreateScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := middleware.GetSessionID(r.Context())
	if sessionID == "" {
		http.Error(w, "Session required", http.StatusUnauthorized)
		return
	}

	if err := r.ParseMultipartForm(h.config.MaxUploadSize); err != nil {
		h.renderErrorFragment(w, "Failed to parse form")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		h.renderErrorFragment(w, "Please select an image to upload")
		return
	}
	defer file.Close()

	mimeType := header.Header.Get("Content-Type")
	if !isValidImageType(mimeType) {
		h.renderErrorFragment(w, "Invalid image type. Please use JPEG, PNG, or WebP.")
		return
	}

	if header.Size > h.config.MaxUploadSize {
		h.renderErrorFragment(w, fmt.Sprintf("File too large. Maximum size is %v MB.", h.config.MaxUploadSize/(1024*1024)))
		return
	}

	imageData, err := io.ReadAll(file)
	if err != nil {
		h.renderErrorFragment(w, "Failed to read uploaded file")
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
		h.renderErrorFragment(w, "Failed to initialize scan")
		return
	}

	storagePath, sha256, err := h.fileStorage.SaveImage(scanID, imageData, mimeType)
	if err != nil {
		log.Printf("Failed to save image: %v", err)
		h.renderErrorFragment(w, "Failed to save uploaded image")
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
		h.renderErrorFragment(w, "Failed to create scan image")
		return
	}

	go h.processOCR(context.Background(), scanID, imageData, mimeType)

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", fmt.Sprintf("/scans/%s", scanID))
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/scans/%s", scanID), http.StatusFound)
}

func (h *Handlers) GetScan(w http.ResponseWriter, r *http.Request) {
	scanID := strings.TrimPrefix(r.URL.Path, "/scans/")
	if scanID == "" {
		http.Error(w, "Scan ID required", http.StatusBadRequest)
		return
	}

	scan, ocrResult, err := h.db.GetScanWithOCR(r.Context(), scanID)
	// #region agent log
	writeDebugLog := func(location, message string, hypothesisId string, data map[string]interface{}) {
		logPath := "/Users/mac/WebApps/projects/gemini-hackathon/.cursor/debug.log"
		dirPath := "/Users/mac/WebApps/projects/gemini-hackathon/.cursor"
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			log.Printf("ERROR: Failed to create .cursor directory: %v", err)
			return
		}
		logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("ERROR: Failed to open debug.log: %v", err)
			return
		}
		defer logFile.Close()
		logData, marshalErr := json.Marshal(map[string]interface{}{
			"sessionId":    "debug-session",
			"runId":         "run1",
			"hypothesisId":   hypothesisId,
			"location":      location,
			"message":       message,
			"data":          data,
			"timestamp":     time.Now().UnixMilli(),
		})
		if marshalErr != nil {
			log.Printf("ERROR: Failed to marshal debug log: %v", marshalErr)
			return
		}
		if _, writeErr := logFile.WriteString(string(logData) + "\n"); writeErr != nil {
			log.Printf("ERROR: Failed to write debug log: %v", writeErr)
			return
		}
		if syncErr := logFile.Sync(); syncErr != nil {
			log.Printf("ERROR: Failed to sync debug log: %v", syncErr)
		}
	}
	if err != nil {
		writeDebugLog("handlers.go:154", "GetScanWithOCR error", "C", map[string]interface{}{"scanID": scanID, "error": err.Error()})
	} else if scan != nil {
		hasOCR := ocrResult != nil
		shouldStopPolling := hasOCR || scan.Status == "failed" || scan.Status == "failed_overloaded" || scan.Status == "failed_auth"
		writeDebugLog("handlers.go:162", "GetScanWithOCR result", "C", map[string]interface{}{"scanID": scanID, "status": scan.Status, "hasOCR": hasOCR, "shouldStopPolling": shouldStopPolling})
		log.Printf("DEBUG: GetScan for %s - status: %q, hasOCR: %v, shouldStopPolling: %v", scanID, scan.Status, hasOCR, shouldStopPolling)
	}
	// #endregion
	if err != nil {
		log.Printf("GetScanWithOCR error for scan %s: %v", scanID, err)
		http.Error(w, "Scan not found", http.StatusNotFound)
		return
	}

	if scan == nil {
		http.Error(w, "Scan not found", http.StatusNotFound)
		return
	}

	data := map[string]interface{}{
		"Scan":      scan,
		"OCRResult": ocrResult,
	}

	if r.Header.Get("HX-Request") == "true" {
		// Note: templates are parsed with their base filename (e.g. "ocr_status.html"),
		// not a path like "fragments/ocr_status.html".
		if err := h.templates.ExecuteTemplate(w, "ocr_status.html", data); err == nil {
			return
		}
	}

	if err := h.templates.ExecuteTemplate(w, "scan", data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handlers) Annotate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	scanID := strings.TrimPrefix(r.URL.Path, "/scans/")
	scanID = strings.TrimSuffix(scanID, "/annotate")
	if scanID == "" {
		http.Error(w, "Scan ID required", http.StatusBadRequest)
		return
	}

	selectedText := r.FormValue("selected_text")
	if strings.TrimSpace(selectedText) == "" {
		h.renderErrorFragment(w, "Please select some text to annotate")
		return
	}

	if len(selectedText) > 1000 {
		h.renderErrorFragment(w, "Selected text is too long")
		return
	}

	ocrResult, err := h.db.GetOCRResult(r.Context(), scanID)
	if err != nil {
		h.renderErrorFragment(w, "Failed to load OCR result")
		return
	}

	annotationResp, err := h.geminiClient.Annotate(r.Context(), ocrResult.RawText, selectedText)
	if err != nil {
		log.Printf("Failed to generate annotation: %v", err)
		h.renderErrorFragment(w, "Failed to generate annotation. Please try again.")
		return
	}

	annotation := &models.Annotation{
		ID:                 generateID(),
		ScanID:             scanID,
		OCRResultID:        ocrResult.ID,
		SelectedText:       selectedText,
		Meaning:            annotationResp.Meaning,
		UsageExample:       annotationResp.UsageExample,
		WhenToUse:          annotationResp.WhenToUse,
		WordBreakdown:      annotationResp.WordBreakdown,
		AlternativeMeanings: annotationResp.AlternativeMeanings,
		Model:              "gemini-2.5-flash",
		PromptVersion:      "1.0",
		CreatedAt:          time.Now(),
	}

	if err := h.db.CreateAnnotation(r.Context(), annotation); err != nil {
		log.Printf("Failed to save annotation: %v", err)
		h.renderErrorFragment(w, "Failed to save annotation")
		return
	}

	// Templates are parsed with their base filename (e.g. "annotation.html").
	if err := h.templates.ExecuteTemplate(w, "annotation.html", annotation); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handlers) GetScanImage(w http.ResponseWriter, r *http.Request) {
	scanID := strings.TrimPrefix(r.URL.Path, "/scans/")
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
		ID:                 generateID(),
		ScanID:             scanID,
		OCRResultID:        ocrResult.ID,
		SelectedText:       selectedText,
		Meaning:            annotationResp.Meaning,
		UsageExample:       annotationResp.UsageExample,
		WhenToUse:          annotationResp.WhenToUse,
		WordBreakdown:      annotationResp.WordBreakdown,
		AlternativeMeanings: annotationResp.AlternativeMeanings,
		Model:              "gemini-2.5-flash",
		PromptVersion:      "1.0",
		CreatedAt:          time.Now(),
	}

	if err := h.db.CreateAnnotation(r.Context(), annotation); err != nil {
		log.Printf("Failed to save annotation: %v", err)
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to save annotation")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(annotation)
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
	// #region agent log
	writeDebugLog := func(location, message string, hypothesisId string, data map[string]interface{}) {
		logPath := "/Users/mac/WebApps/projects/gemini-hackathon/.cursor/debug.log"
		dirPath := "/Users/mac/WebApps/projects/gemini-hackathon/.cursor"
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			log.Printf("ERROR: Failed to create .cursor directory: %v", err)
			return
		}
		logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("ERROR: Failed to open debug.log: %v", err)
			return
		}
		defer logFile.Close()
		logData, marshalErr := json.Marshal(map[string]interface{}{
			"sessionId":    "debug-session",
			"runId":         "run1",
			"hypothesisId":   hypothesisId,
			"location":      location,
			"message":       message,
			"data":          data,
			"timestamp":     time.Now().UnixMilli(),
		})
		if marshalErr != nil {
			log.Printf("ERROR: Failed to marshal debug log: %v", marshalErr)
			return
		}
		if _, writeErr := logFile.WriteString(string(logData) + "\n"); writeErr != nil {
			log.Printf("ERROR: Failed to write debug log: %v", writeErr)
			return
		}
		if syncErr := logFile.Sync(); syncErr != nil {
			log.Printf("ERROR: Failed to sync debug log: %v", syncErr)
		}
	}
	writeDebugLog("handlers.go:359", "processOCR entry", "A", map[string]interface{}{"scanID": scanID})
	// #endregion
	log.Printf("Starting OCR processing for scan %s", scanID)
	// #region agent log
	writeDebugLog("handlers.go:362", "Before OCR call", "A", map[string]interface{}{"scanID": scanID, "imageDataLen": len(imageData)})
	// #endregion
	ocrResp, err := h.geminiClient.OCR(ctx, imageData, mimeType)
	// #region agent log
	if err != nil {
		writeDebugLog("handlers.go:365", "OCR call returned with error", "A", map[string]interface{}{"scanID": scanID, "errorOccurred": true})
	} else {
		writeDebugLog("handlers.go:365", "OCR call returned successfully", "A", map[string]interface{}{"scanID": scanID, "errorOccurred": false, "responseLen": len(ocrResp.RawText)})
	}
	// #endregion
	if err != nil {
		// #region agent log
		errMsg := err.Error()
		writeDebugLog("handlers.go:301", "OCR error received", "A", map[string]interface{}{"scanID": scanID, "errorMsg": errMsg, "errorType": fmt.Sprintf("%T", err)})
		// #endregion
		log.Printf("OCR failed for scan %s: %v", scanID, err)
		status := "failed"
		
		errMsgLower := strings.ToLower(errMsg)
		// #region agent log
		hasOverloaded := strings.Contains(errMsg, "503") ||
			strings.Contains(errMsg, "429") ||
			strings.Contains(errMsgLower, "overloaded") ||
			strings.Contains(errMsg, "UNAVAILABLE") ||
			strings.Contains(errMsgLower, "unavailable") ||
			strings.Contains(errMsg, "RESOURCE_EXHAUSTED") ||
			strings.Contains(errMsgLower, "quota")
		hasAuth := strings.Contains(errMsg, "401") || strings.Contains(errMsg, "403") || strings.Contains(errMsgLower, "api key") || strings.Contains(errMsg, "not initialized")
		writeDebugLog("handlers.go:311", "Error pattern check", "A", map[string]interface{}{"scanID": scanID, "hasOverloaded": hasOverloaded, "hasAuth": hasAuth, "errMsg": errMsg, "errMsgLower": errMsgLower})
		// #endregion
		log.Printf("DEBUG: Error message for scan %s: %q (lowercase: %q)", scanID, errMsg, errMsgLower)
		if hasOverloaded {
			status = "failed_overloaded"
			log.Printf("DEBUG: Setting status to failed_overloaded for scan %s", scanID)
		} else if strings.Contains(errMsg, "401") || strings.Contains(errMsg, "403") || strings.Contains(errMsgLower, "api key") || strings.Contains(errMsg, "not initialized") {
			status = "failed_auth"
			log.Printf("DEBUG: Setting status to failed_auth for scan %s", scanID)
		} else {
			log.Printf("DEBUG: No pattern matched for scan %s, keeping status as 'failed'", scanID)
		}
		
		// #region agent log
		writeDebugLog("handlers.go:329", "Before status update", "B", map[string]interface{}{"scanID": scanID, "status": status})
		// #endregion
		log.Printf("DEBUG: Attempting to update scan %s status to %s", scanID, status)
		if updateErr := h.db.UpdateScanStatus(ctx, scanID, status); updateErr != nil {
			// #region agent log
			writeDebugLog("handlers.go:334", "Status update failed", "B", map[string]interface{}{"scanID": scanID, "updateErr": updateErr.Error()})
			// #endregion
			log.Printf("ERROR: Failed to update scan status to %s for scan %s: %v", status, scanID, updateErr)
		} else {
			// #region agent log
			writeDebugLog("handlers.go:340", "Status update succeeded", "B", map[string]interface{}{"scanID": scanID, "status": status})
			// #endregion
			log.Printf("DEBUG: Successfully updated scan %s status to %s", scanID, status)
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

func (h *Handlers) renderErrorFragment(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusBadRequest)
	data := map[string]interface{}{
		"Message":   message,
		"Retryable": true,
	}
	// Templates are parsed with their base filename (e.g. "error.html").
	if err := h.templates.ExecuteTemplate(w, "error.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func loadTemplates() (*template.Template, error) {
	baseDir := findProjectRoot()
	if baseDir == "" {
		return nil, fmt.Errorf("could not find project root")
	}

	patterns := []string{
		filepath.Join(baseDir, "web/templates/base.html"),
		filepath.Join(baseDir, "web/templates/pages/*.html"),
		filepath.Join(baseDir, "web/templates/fragments/*.html"),
		filepath.Join(baseDir, "web/templates/components/*.html"),
	}

	var files []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to glob pattern %s: %w", pattern, err)
		}
		files = append(files, matches...)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no template files found")
	}

	tmpl, err := template.ParseFiles(files...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	return tmpl, nil
}

func findProjectRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}

	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}

package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Failed to get file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	mimeType := header.Header.Get("Content-Type")
	if !isValidImageType(mimeType) {
		http.Error(w, "Invalid image type", http.StatusBadRequest)
		return
	}

	if header.Size > h.config.MaxUploadSize {
		http.Error(w, "File too large", http.StatusRequestEntityTooLarge)
		return
	}

	imageData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
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
		http.Error(w, "Failed to create scan", http.StatusInternalServerError)
		return
	}

	storagePath, sha256, err := h.fileStorage.SaveImage(scanID, imageData, mimeType)
	if err != nil {
		log.Printf("Failed to save image: %v", err)
		http.Error(w, "Failed to save image", http.StatusInternalServerError)
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
		http.Error(w, "Failed to create scan image", http.StatusInternalServerError)
		return
	}

	go h.processOCR(context.Background(), scanID, imageData, mimeType)

	http.Redirect(w, r, fmt.Sprintf("/scans/%s", scanID), http.StatusFound)
}

func (h *Handlers) GetScan(w http.ResponseWriter, r *http.Request) {
	scanID := strings.TrimPrefix(r.URL.Path, "/scans/")
	if scanID == "" {
		http.Error(w, "Scan ID required", http.StatusBadRequest)
		return
	}

	scan, ocrResult, err := h.db.GetScanWithOCR(r.Context(), scanID)
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
		if err := h.templates.ExecuteTemplate(w, "fragments/ocr_status.html", data); err == nil {
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
		Model:              "gemini-1.5-flash",
		PromptVersion:      "1.0",
		CreatedAt:          time.Now(),
	}

	if err := h.db.CreateAnnotation(r.Context(), annotation); err != nil {
		log.Printf("Failed to save annotation: %v", err)
		h.renderErrorFragment(w, "Failed to save annotation")
		return
	}

	if err := h.templates.ExecuteTemplate(w, "fragments/annotation.html", annotation); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handlers) Healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (h *Handlers) processOCR(ctx context.Context, scanID string, imageData []byte, mimeType string) {
	log.Printf("Starting OCR processing for scan %s", scanID)
	ocrResp, err := h.geminiClient.OCR(ctx, imageData, mimeType)
	if err != nil {
		log.Printf("OCR failed for scan %s: %v", scanID, err)
		if updateErr := h.db.UpdateScanStatus(ctx, scanID, "failed"); updateErr != nil {
			log.Printf("Failed to update scan status to failed: %v", updateErr)
		}
		return
	}
	log.Printf("OCR successful for scan %s, extracted %d characters", scanID, len(ocrResp.RawText))

	ocrResult := &models.OCRResult{
		ID:             generateID(),
		ScanID:         scanID,
		Model:          "gemini-1.5-flash",
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
	data := map[string]interface{}{
		"Message":   message,
		"Retryable": true,
	}
	if err := h.templates.ExecuteTemplate(w, "fragments/error.html", data); err != nil {
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

package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/storage"
)

type AnnotationHandlers struct {
	db     storage.DB
	config *config.Config
}

func NewAnnotationHandlers(db storage.DB, cfg *config.Config) *AnnotationHandlers {
	return &AnnotationHandlers{
		db:     db,
		config: cfg,
	}
}

type CreateAnnotationRequest struct {
	ScanID          int64             `json:"scanId"`
	HighlightedText string            `json:"highlightedText"`
	ContextText     string            `json:"contextText"`
	NuanceData      models.NuanceData `json:"nuanceData"`
}

type CreateAnnotationResponse struct {
	AnnotationID int64  `json:"annotationId"`
	Status       string `json:"status"`
}

type AnnotationListItem struct {
	ID              int64  `json:"id"`
	HighlightedText string `json:"highlightedText"`
	NuanceSummary   string `json:"nuanceSummary"`
	CreatedAt       string `json:"createdAt"`
}

type GetAnnotationsResponse struct {
	Data []AnnotationListItem `json:"data"`
	Meta PaginationMeta       `json:"meta"`
}

func (h *AnnotationHandlers) CreateAnnotationAPI(w http.ResponseWriter, r *http.Request) {
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

	var req CreateAnnotationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.HighlightedText == "" {
		h.writeJSONError(w, http.StatusBadRequest, "highlightedText is required")
		return
	}

	var scanID *int64
	if req.ScanID > 0 {
		scanID = &req.ScanID
	}

	annotation := &models.Annotation{
		UserID:          userID,
		ScanID:          scanID,
		HighlightedText: req.HighlightedText,
		ContextText:     &req.ContextText,
		NuanceData:      req.NuanceData,
		IsBookmarked:    true,
		CreatedAt:       time.Now(),
	}

	annotationID, err := h.db.CreateAnnotation(r.Context(), annotation)
	if err != nil {
		log.Printf("Failed to create annotation: %v", err)
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to create annotation")
		return
	}

	response := CreateAnnotationResponse{
		AnnotationID: annotationID,
		Status:       "saved",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func (h *AnnotationHandlers) GetAnnotationsAPI(w http.ResponseWriter, r *http.Request) {
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

	annotations, err := h.db.GetAnnotationsByUserID(r.Context(), userID, page, size)
	if err != nil {
		log.Printf("Failed to get annotations: %v", err)
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to get annotations")
		return
	}

	data := make([]AnnotationListItem, len(annotations))
	for i, ann := range annotations {
		summary := summarizeNuance(ann.NuanceData)
		data[i] = AnnotationListItem{
			ID:              ann.ID,
			HighlightedText: ann.HighlightedText,
			NuanceSummary:   summary,
			CreatedAt:       ann.CreatedAt.Format(time.RFC3339),
		}
	}

	var nextPage, prevPage *int
	if len(annotations) == size {
		nextPageVal := page + 1
		nextPage = &nextPageVal
	}
	if page > 1 {
		prevPageVal := page - 1
		prevPage = &prevPageVal
	}

	response := GetAnnotationsResponse{
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

func (h *AnnotationHandlers) setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, x-token")
}

func (h *AnnotationHandlers) writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
	})
}

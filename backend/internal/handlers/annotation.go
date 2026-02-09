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

type GetAnnotationResponse struct {
	ID              int64             `json:"id"`
	HighlightedText string            `json:"highlightedText"`
	ContextText     string            `json:"contextText,omitempty"`
	NuanceData      models.NuanceData `json:"nuanceData"`
	CreatedAt       string            `json:"createdAt"`
}

type GetAnnotationsResponse struct {
	Data []AnnotationListItem `json:"data"`
	Meta PaginationMeta       `json:"meta"`
}

func (h *AnnotationHandlers) CreateAnnotationAPI(w http.ResponseWriter, r *http.Request) {
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

func (h *AnnotationHandlers) GetAnnotationAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Extract annotation ID from path: /v1/annotations/{id}
	path := r.URL.Path
	idStr := path[len("/v1/annotations/"):]
	annotationID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid annotation ID")
		return
	}

	annotation, err := h.db.GetAnnotationByID(r.Context(), annotationID)
	if err != nil {
		log.Printf("Failed to get annotation: %v", err)
		h.writeJSONError(w, http.StatusNotFound, "Annotation not found")
		return
	}

	// Verify the annotation belongs to the user
	if annotation.UserID != userID {
		h.writeJSONError(w, http.StatusForbidden, "Access denied")
		return
	}

	contextText := ""
	if annotation.ContextText != nil {
		contextText = *annotation.ContextText
	}

	response := GetAnnotationResponse{
		ID:              annotation.ID,
		HighlightedText: annotation.HighlightedText,
		ContextText:     contextText,
		NuanceData:      annotation.NuanceData,
		CreatedAt:       annotation.CreatedAt.Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *AnnotationHandlers) GetAnnotationsAPI(w http.ResponseWriter, r *http.Request) {
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

	scanIDParam := r.URL.Query().Get("scanId")
	var annotations []*models.Annotation
	var err error
	if scanIDParam == "" {
		annotations, err = h.db.GetAnnotationsByUserID(r.Context(), userID, page, size)
	} else {
		scanID, parseErr := strconv.ParseInt(scanIDParam, 10, 64)
		if parseErr != nil || scanID <= 0 {
			h.writeJSONError(w, http.StatusBadRequest, "scanId must be a positive integer")
			return
		}
		annotations, err = h.db.GetAnnotationsByUserIDAndScanID(r.Context(), userID, scanID, page, size)
	}
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

func (h *AnnotationHandlers) writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
	})
}

func (h *AnnotationHandlers) AnnotationsAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.CreateAnnotationAPI(w, r)
	case http.MethodGet:
		h.GetAnnotationsAPI(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

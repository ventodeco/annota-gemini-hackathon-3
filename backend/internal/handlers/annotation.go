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
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/services"
	"github.com/gemini-hackathon/app/internal/storage"
)

type AnnotationHandlers struct {
	db          storage.DB
	config      *config.Config
	annotations *services.AnnotationService
}

func NewAnnotationHandlers(db storage.DB, cfg *config.Config) *AnnotationHandlers {
	return &AnnotationHandlers{
		db:          db,
		config:      cfg,
		annotations: services.NewAnnotationService(db, nil),
	}
}

// WithAIClient lets saves keep echoed provenance that matches the client's
// current annotation producer. Without it, saved annotations have no provenance.
func (h *AnnotationHandlers) WithAIClient(client ai.Client) *AnnotationHandlers {
	h.annotations = services.NewAnnotationService(h.db, client)
	return h
}

type CreateAnnotationRequest struct {
	ScanID          int64                  `json:"scanId"`
	HighlightedText string                 `json:"highlightedText"`
	ContextText     string                 `json:"contextText"`
	NuanceData      createAnnotationNuance `json:"nuanceData"`
}

// createAnnotationNuance reads echoed provenance as raw JSON so a malformed
// value is dropped instead of rejecting the save; the outer field shadows the
// embedded NuanceData.Provenance during decoding.
type createAnnotationNuance struct {
	models.NuanceData
	Provenance json.RawMessage `json:"provenance,omitempty"`
}

type CreateAnnotationResponse struct {
	AnnotationID int64  `json:"annotationId"`
	Status       string `json:"status"`
}

type AnnotationListItem struct {
	ID              int64  `json:"id"`
	HighlightedText string `json:"highlightedText"`
	NuanceSummary   string `json:"nuanceSummary"`
	ScanID          *int64 `json:"scanId,omitempty"`
	SourceType      string `json:"sourceType,omitempty"`
	DocumentID      *int64 `json:"documentId,omitempty"`
	PageNumber      *int   `json:"pageNumber,omitempty"`
	SourceLabel     string `json:"sourceLabel,omitempty"`
	CreatedAt       string `json:"createdAt"`
}

type GetAnnotationResponse struct {
	ID              int64             `json:"id"`
	ScanID          *int64            `json:"scanId,omitempty"`
	SourceType      string            `json:"sourceType,omitempty"`
	DocumentID      *int64            `json:"documentId,omitempty"`
	PageNumber      *int              `json:"pageNumber,omitempty"`
	SourceLabel     string            `json:"sourceLabel,omitempty"`
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
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req CreateAnnotationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.HighlightedText == "" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "highlightedText is required")
		return
	}

	annotationID, err := h.annotations.Create(r.Context(), userID, services.NewAnnotation{
		ScanID:           req.ScanID,
		HighlightedText:  req.HighlightedText,
		ContextText:      req.ContextText,
		NuanceData:       req.NuanceData.NuanceData,
		EchoedProvenance: req.NuanceData.Provenance,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrScanNotFound):
			httputil.WriteJSONError(w, http.StatusNotFound, "Scan not found")
		case errors.Is(err, services.ErrScanForbidden):
			httputil.WriteJSONError(w, http.StatusForbidden, "Access denied")
		default:
			logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context())).ErrorWithErr(err, "Failed to create annotation")
			httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to create annotation")
		}
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

func (h *AnnotationHandlers) AnnotationByIDAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getAnnotationHandler(w, r)
	case http.MethodDelete:
		h.deleteAnnotationHandler(w, r)
	default:
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *AnnotationHandlers) deleteAnnotationHandler(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	path := r.URL.Path
	idStr := strings.TrimSuffix(path[len("/v1/annotations/"):], "/")
	annotationID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || annotationID <= 0 {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid annotation ID")
		return
	}

	if err := h.annotations.Delete(r.Context(), userID, annotationID); err != nil {
		logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context())).ErrorWithErr(err, "Failed to delete annotation")
		httputil.WriteJSONError(w, http.StatusNotFound, "Annotation not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AnnotationHandlers) getAnnotationHandler(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	path := r.URL.Path
	idStr := strings.TrimSuffix(path[len("/v1/annotations/"):], "/")
	annotationID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid annotation ID")
		return
	}

	annotation, err := h.annotations.Get(r.Context(), userID, annotationID)
	if err != nil {
		if errors.Is(err, services.ErrAnnotationDenied) {
			httputil.WriteJSONError(w, http.StatusForbidden, "Access denied")
			return
		}
		logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context())).ErrorWithErr(err, "Failed to get annotation")
		httputil.WriteJSONError(w, http.StatusNotFound, "Annotation not found")
		return
	}

	contextText := ""
	if annotation.ContextText != nil {
		contextText = *annotation.ContextText
	}

	response := GetAnnotationResponse{
		ID:              annotation.ID,
		ScanID:          annotation.ScanID,
		SourceType:      stringValue(annotation.SourceType),
		DocumentID:      annotation.DocumentID,
		PageNumber:      annotation.PageNumber,
		SourceLabel:     stringValue(annotation.SourceLabel),
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
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	page, size := httputil.ParsePagination(r, h.config.DefaultPageSize)

	scanIDParam := r.URL.Query().Get("scanId")
	documentIDParam := r.URL.Query().Get("documentId")
	pageNumberParam := r.URL.Query().Get("pageNumber")
	var filter services.AnnotationFilter
	switch {
	case scanIDParam != "":
		scanID, parseErr := strconv.ParseInt(scanIDParam, 10, 64)
		if parseErr != nil || scanID <= 0 {
			httputil.WriteJSONError(w, http.StatusBadRequest, "scanId must be a positive integer")
			return
		}
		filter.ScanID = scanID
	case documentIDParam != "":
		documentID, parseErr := strconv.ParseInt(documentIDParam, 10, 64)
		if parseErr != nil || documentID <= 0 {
			httputil.WriteJSONError(w, http.StatusBadRequest, "documentId must be a positive integer")
			return
		}
		filter.DocumentID = documentID
		if pageNumberParam != "" {
			parsedPage, pageErr := strconv.Atoi(pageNumberParam)
			if pageErr != nil || parsedPage <= 0 {
				httputil.WriteJSONError(w, http.StatusBadRequest, "pageNumber must be a positive integer")
				return
			}
			filter.PageNumber = &parsedPage
		}
	}
	annotations, err := h.annotations.List(r.Context(), userID, filter, page, size)
	if err != nil {
		logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context())).ErrorWithErr(err, "Failed to get annotations")
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to get annotations")
		return
	}

	data := make([]AnnotationListItem, len(annotations))
	for i, ann := range annotations {
		summary := summarizeNuance(ann.NuanceData)
		data[i] = AnnotationListItem{
			ID:              ann.ID,
			HighlightedText: ann.HighlightedText,
			NuanceSummary:   summary,
			ScanID:          ann.ScanID,
			SourceType:      stringValue(ann.SourceType),
			DocumentID:      ann.DocumentID,
			PageNumber:      ann.PageNumber,
			SourceLabel:     stringValue(ann.SourceLabel),
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

func (h *AnnotationHandlers) AnnotationsAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.CreateAnnotationAPI(w, r)
	case http.MethodGet:
		h.GetAnnotationsAPI(w, r)
	default:
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

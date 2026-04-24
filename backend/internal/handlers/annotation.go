package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/httputil"
	"github.com/gemini-hackathon/app/internal/logger"
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

	var scanID *int64
	var sourceType *string
	var documentID *int64
	var pageNumber *int
	var sourceLabel *string
	if req.ScanID > 0 {
		scanID = &req.ScanID
		scan, err := h.db.GetScanByID(r.Context(), req.ScanID)
		if err != nil || scan == nil {
			httputil.WriteJSONError(w, http.StatusNotFound, "Scan not found")
			return
		}
		if scan.UserID != userID {
			httputil.WriteJSONError(w, http.StatusForbidden, "Access denied")
			return
		}
		sourceTypeValue := scan.SourceType
		if sourceTypeValue == "" {
			if scan.DocumentID == nil {
				sourceTypeValue = "image"
			} else {
				sourceTypeValue = "pdf"
			}
		}
		sourceType = &sourceTypeValue
		documentID = scan.DocumentID
		pageNumber = scan.PageNumber
		label := buildSourceLabel(r.Context(), h.db, scan)
		sourceLabel = &label
	}

	annotation := &models.Annotation{
		UserID:          userID,
		ScanID:          scanID,
		SourceType:      sourceType,
		DocumentID:      documentID,
		PageNumber:      pageNumber,
		SourceLabel:     sourceLabel,
		HighlightedText: req.HighlightedText,
		ContextText:     &req.ContextText,
		NuanceData:      req.NuanceData,
		IsBookmarked:    true,
		CreatedAt:       time.Now(),
	}

	annotationID, err := h.db.CreateAnnotation(r.Context(), annotation)
	if err != nil {
		logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context())).ErrorWithErr(err, "Failed to create annotation")
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to create annotation")
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

	if err := h.db.DeleteAnnotation(r.Context(), annotationID, userID); err != nil {
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

	annotation, err := h.db.GetAnnotationByID(r.Context(), annotationID)
	if err != nil {
		logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context())).ErrorWithErr(err, "Failed to get annotation")
		httputil.WriteJSONError(w, http.StatusNotFound, "Annotation not found")
		return
	}

	if annotation.UserID != userID {
		httputil.WriteJSONError(w, http.StatusForbidden, "Access denied")
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
	var annotations []*models.Annotation
	var err error
	switch {
	case scanIDParam != "":
		scanID, parseErr := strconv.ParseInt(scanIDParam, 10, 64)
		if parseErr != nil || scanID <= 0 {
			httputil.WriteJSONError(w, http.StatusBadRequest, "scanId must be a positive integer")
			return
		}
		annotations, err = h.db.GetAnnotationsByUserIDAndScanID(r.Context(), userID, scanID, page, size)
	case documentIDParam != "":
		documentID, parseErr := strconv.ParseInt(documentIDParam, 10, 64)
		if parseErr != nil || documentID <= 0 {
			httputil.WriteJSONError(w, http.StatusBadRequest, "documentId must be a positive integer")
			return
		}
		var pageNumber *int
		if pageNumberParam != "" {
			parsedPage, pageErr := strconv.Atoi(pageNumberParam)
			if pageErr != nil || parsedPage <= 0 {
				httputil.WriteJSONError(w, http.StatusBadRequest, "pageNumber must be a positive integer")
				return
			}
			pageNumber = &parsedPage
		}
		annotations, err = h.db.GetAnnotationsByUserIDAndDocumentPage(r.Context(), userID, documentID, pageNumber, page, size)
	default:
		annotations, err = h.db.GetAnnotationsByUserID(r.Context(), userID, page, size)
	}
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

func buildSourceLabel(ctx context.Context, db storage.DB, scan *models.Scan) string {
	if scan.DocumentID != nil {
		page := ""
		if scan.PageNumber != nil {
			page = " page " + strconv.Itoa(*scan.PageNumber)
		}
		doc, err := db.GetDocumentByID(ctx, *scan.DocumentID)
		if err == nil && doc != nil && doc.Filename != "" {
			return doc.Filename + page
		}
		return "PDF Document #" + strconv.FormatInt(*scan.DocumentID, 10) + page
	}
	return "OCR Scan #" + strconv.FormatInt(scan.ID, 10)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

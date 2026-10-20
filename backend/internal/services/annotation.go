package services

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/gemini-hackathon/app/internal/ai"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/storage"
)

// AnnotationService saves, reads, lists and deletes a user's annotations.
type AnnotationService struct {
	db       storage.DB
	aiClient ai.Client
}

// NewAnnotationService returns the service. aiClient may be nil; then saved
// annotations have no provenance.
func NewAnnotationService(db storage.DB, aiClient ai.Client) *AnnotationService {
	return &AnnotationService{db: db, aiClient: aiClient}
}

// NewAnnotation is a save request. ScanID 0 means no source scan.
// EchoedProvenance is the provenance the client got back from analyze.
type NewAnnotation struct {
	ScanID           int64
	HighlightedText  string
	ContextText      string
	NuanceData       models.NuanceData
	EchoedProvenance json.RawMessage
}

// AnnotationFilter selects which annotations List returns: by scan, by
// document (optionally one page), or all when both IDs are zero.
type AnnotationFilter struct {
	ScanID     int64
	DocumentID int64
	PageNumber *int
}

// Create saves a bookmarked annotation. A source scan must belong to the
// user (ErrScanNotFound / ErrScanForbidden); its source type, document page
// and label are copied so the annotation outlives the source.
func (s *AnnotationService) Create(ctx context.Context, userID int64, in NewAnnotation) (int64, error) {
	var scanID *int64
	var sourceType *string
	var documentID *int64
	var pageNumber *int
	var sourceLabel *string
	if in.ScanID > 0 {
		scanID = &in.ScanID
		scan, err := s.db.GetScanByID(ctx, in.ScanID)
		if err != nil || scan == nil {
			return 0, ErrScanNotFound
		}
		if scan.UserID != userID {
			return 0, ErrScanForbidden
		}
		sourceTypeValue := SourceTypeOrDefault(scan)
		sourceType = &sourceTypeValue
		documentID = scan.DocumentID
		pageNumber = scan.PageNumber
		label := s.sourceLabel(ctx, scan)
		sourceLabel = &label
	}

	nuanceData := in.NuanceData
	nuanceData.Provenance = s.acceptedProvenance(in.EchoedProvenance)

	contextText := in.ContextText
	return s.db.CreateAnnotation(ctx, &models.Annotation{
		UserID:          userID,
		ScanID:          scanID,
		SourceType:      sourceType,
		DocumentID:      documentID,
		PageNumber:      pageNumber,
		SourceLabel:     sourceLabel,
		HighlightedText: in.HighlightedText,
		ContextText:     &contextText,
		NuanceData:      nuanceData,
		IsBookmarked:    true,
		CreatedAt:       time.Now(),
	})
}

// Get returns the user's annotation, ErrAnnotationNotFound (wrapping any
// lookup error), or ErrAnnotationDenied.
func (s *AnnotationService) Get(ctx context.Context, userID, annotationID int64) (*models.Annotation, error) {
	annotation, err := s.db.GetAnnotationByID(ctx, annotationID)
	if err != nil {
		return nil, wrapNotFound(ErrAnnotationNotFound, err)
	}
	if annotation == nil {
		return nil, ErrAnnotationNotFound
	}
	if annotation.UserID != userID {
		return nil, ErrAnnotationDenied
	}
	return annotation, nil
}

// Delete removes the user's annotation; any failure reads as not found.
func (s *AnnotationService) Delete(ctx context.Context, userID, annotationID int64) error {
	if err := s.db.DeleteAnnotation(ctx, annotationID, userID); err != nil {
		return wrapNotFound(ErrAnnotationNotFound, err)
	}
	return nil
}

// List returns a page of the user's annotations matching filter.
func (s *AnnotationService) List(ctx context.Context, userID int64, filter AnnotationFilter, page, size int) ([]*models.Annotation, error) {
	switch {
	case filter.ScanID > 0:
		return s.db.GetAnnotationsByUserIDAndScanID(ctx, userID, filter.ScanID, page, size)
	case filter.DocumentID > 0:
		return s.db.GetAnnotationsByUserIDAndDocumentPage(ctx, userID, filter.DocumentID, filter.PageNumber, page, size)
	default:
		return s.db.GetAnnotationsByUserID(ctx, userID, page, size)
	}
}

// acceptedProvenance keeps echoed provenance only when its configured fields
// exactly match the server's current annotation producer, so arbitrary client
// values (keys, request IDs, text) are never stored. A producer change between
// analyze and save, or any mismatch, leaves the provenance unknown. The server
// cannot tie an echoed reportedModel to the saved output, so saved annotations
// never record one: it stays unknown even when the analyze response showed it.
func (s *AnnotationService) acceptedProvenance(raw json.RawMessage) *models.AIProvenance {
	current := ProvenanceFor(s.aiClient, "annotation", nil)
	if current == nil || len(raw) == 0 {
		return nil
	}
	var echoed models.AIProvenance
	if err := json.Unmarshal(raw, &echoed); err != nil {
		return nil
	}
	echoed.ReportedModel = ""
	if echoed != *current {
		return nil
	}
	return current
}

func (s *AnnotationService) sourceLabel(ctx context.Context, scan *models.Scan) string {
	if scan.DocumentID != nil {
		page := ""
		if scan.PageNumber != nil {
			page = " page " + strconv.Itoa(*scan.PageNumber)
		}
		doc, err := s.db.GetDocumentByID(ctx, *scan.DocumentID)
		if err == nil && doc != nil && doc.Filename != "" {
			return doc.Filename + page
		}
		return "PDF Document #" + strconv.FormatInt(*scan.DocumentID, 10) + page
	}
	return "OCR Scan #" + strconv.FormatInt(scan.ID, 10)
}

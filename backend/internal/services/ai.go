package services

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/gemini-hackathon/app/internal/ai"
	"github.com/gemini-hackathon/app/internal/knowledge"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/storage"
	"github.com/gemini-hackathon/app/internal/usage"
)

// analyzeContextMaxRunes bounds the scan text sent as analysis context.
const analyzeContextMaxRunes = 12000

// AIService runs metered, synchronous AI work: text analysis and speech.
type AIService struct {
	db        storage.DB
	aiClient  ai.Client
	knowledge knowledge.Service
	meter     *usage.Meter
}

func NewAIService(db storage.DB, aiClient ai.Client, knowledgeSvc knowledge.Service, meter *usage.Meter) *AIService {
	return &AIService{db: db, aiClient: aiClient, knowledge: knowledgeSvc, meter: meter}
}

// AnalyzeRequest is a text analysis request. When ScanID names one of the
// user's scans with OCR text, that text replaces Context.
type AnalyzeRequest struct {
	Text    string
	Context string
	ScanID  *int64
}

// Analysis is an annotation and the provenance of the call that produced it.
type Analysis struct {
	Annotation *ai.AnnotationResponse
	Provenance *models.AIProvenance
}

// Analyze explains the selected text. It returns ErrUserNotFound, ErrStorage,
// ErrScanNotFound, ErrScanForbidden or *AdmissionError before any provider
// call; a provider failure is returned unchanged after the explanation is
// settled.
func (s *AIService) Analyze(ctx context.Context, userID int64, req AnalyzeRequest) (*Analysis, error) {
	user, err := s.db.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("%w: get user: %w", ErrStorage, err)
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	entries := s.knowledge.Lookup(req.Text)
	contextText, err := s.analyzeContext(ctx, userID, req)
	if err != nil {
		return nil, err
	}

	usageCtx, reservation, err := s.meter.Reserve(ctx, userID, usage.FeatureExplanation, 1)
	if err != nil {
		return nil, &AdmissionError{Err: err}
	}

	resp, err := s.aiClient.AnnotateWithKnowledge(usageCtx, contextText, req.Text, entries)
	s.meter.Finish(usageCtx, reservation, err)
	if err != nil {
		return nil, err
	}
	return &Analysis{
		Annotation: resp,
		Provenance: ProvenanceFor(s.aiClient, "annotation", usage.Calls(usageCtx)),
	}, nil
}

// Speak synthesizes the highlighted text, charged per character spoken;
// context is not spoken. Admission failures are *AdmissionError; a provider
// failure is returned unchanged after the reservation is settled.
func (s *AIService) Speak(ctx context.Context, userID int64, highlightedText, contextText string) (*ai.SpeechResponse, error) {
	units := int64(utf8.RuneCountInString(highlightedText))
	usageCtx, reservation, err := s.meter.Reserve(ctx, userID, usage.FeatureSpeech, units)
	if err != nil {
		return nil, &AdmissionError{Err: err}
	}

	resp, err := s.aiClient.SynthesizeSpeech(usageCtx, highlightedText, contextText)
	s.meter.Finish(usageCtx, reservation, err)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *AIService) analyzeContext(ctx context.Context, userID int64, req AnalyzeRequest) (string, error) {
	if req.ScanID == nil || *req.ScanID <= 0 {
		return req.Context, nil
	}

	scan, err := s.db.GetScanByID(ctx, *req.ScanID)
	if err != nil || scan == nil {
		return "", ErrScanNotFound
	}
	if scan.UserID != userID {
		return "", ErrScanForbidden
	}
	if scan.FullOCRText == nil || *scan.FullOCRText == "" {
		return req.Context, nil
	}
	return contextWindow(*scan.FullOCRText, req.Text, analyzeContextMaxRunes), nil
}

func contextWindow(fullText, selectedText string, maxRunes int) string {
	full := []rune(fullText)
	if len(full) <= maxRunes {
		return fullText
	}

	selectedIndex := strings.Index(fullText, selectedText)
	if selectedIndex < 0 {
		return string(full[:maxRunes])
	}

	center := len([]rune(fullText[:selectedIndex]))
	start := center - maxRunes/2
	if start < 0 {
		start = 0
	}
	end := start + maxRunes
	if end > len(full) {
		end = len(full)
		start = end - maxRunes
		if start < 0 {
			start = 0
		}
	}
	return string(full[start:end])
}

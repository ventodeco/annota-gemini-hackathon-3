package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gemini-hackathon/app/internal/gemini"
	"github.com/gemini-hackathon/app/internal/httputil"
	"github.com/gemini-hackathon/app/internal/knowledge"
	"github.com/gemini-hackathon/app/internal/logger"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/storage"
)

type AIHandlers struct {
	db           storage.DB
	geminiClient gemini.Client
	knowledge    knowledge.Service
}

func NewAIHandlers(db storage.DB, geminiClient gemini.Client, knowledgeSvc knowledge.Service) *AIHandlers {
	return &AIHandlers{
		db:           db,
		geminiClient: geminiClient,
		knowledge:    knowledgeSvc,
	}
}

type AnalyzeRequest struct {
	TextToAnalyze string `json:"textToAnalyze"`
	Context       string `json:"context"`
}

type AnalyzeResponse struct {
	Meaning            string `json:"meaning"`
	UsageExample       string `json:"usageExample"`
	UsageTiming        string `json:"usageTiming"`
	WordBreakdown      string `json:"wordBreakdown"`
	AlternativeMeaning string `json:"alternativeMeaning"`
}

type NuanceSummary struct {
	Meaning string `json:"meaning"`
}

type SpeakRequest struct {
	HighlightedText string `json:"highlightedText"`
	ContextText     string `json:"contextText,omitempty"`
	Tone            string `json:"tone,omitempty"`
}

func (h *AIHandlers) AnalyzeAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.TextToAnalyze == "" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "textToAnalyze is required")
		return
	}

	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil {
		logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context())).ErrorWithErr(err, "Failed to get user")
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Database error")
		return
	}

	if user == nil {
		httputil.WriteJSONError(w, http.StatusNotFound, "User not found")
		return
	}

	targetLanguage := user.PreferredLanguage
	if targetLanguage == "" {
		targetLanguage = "ID"
	}

	// Lookup knowledge context for the selected text
	entries := h.knowledge.Lookup(req.TextToAnalyze)

	// Call Gemini with knowledge context
	resp, err := h.geminiClient.AnnotateWithKnowledge(r.Context(), req.Context, req.TextToAnalyze, entries)
	if err != nil {
		logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context())).ErrorWithErr(err, "Failed to generate annotation")
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to analyze text")
		return
	}

	response := AnalyzeResponse{
		Meaning:            resp.Meaning,
		UsageExample:       resp.UsageExample,
		UsageTiming:        resp.WhenToUse,
		WordBreakdown:      resp.WordBreakdown,
		AlternativeMeaning: resp.AlternativeMeanings,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *AIHandlers) AnalyzeWithLanguageAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.TextToAnalyze == "" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "textToAnalyze is required")
		return
	}

	// Lookup knowledge context for the selected text
	entries := h.knowledge.Lookup(req.TextToAnalyze)

	resp, err := h.geminiClient.AnnotateWithKnowledge(r.Context(), req.Context, req.TextToAnalyze, entries)
	if err != nil {
		logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context())).ErrorWithErr(err, "Failed to generate annotation with language")
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to analyze text")
		return
	}

	response := AnalyzeResponse{
		Meaning:            resp.Meaning,
		UsageExample:       resp.UsageExample,
		UsageTiming:        resp.WhenToUse,
		WordBreakdown:      resp.WordBreakdown,
		AlternativeMeaning: resp.AlternativeMeanings,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *AIHandlers) SpeakAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req SpeakRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.HighlightedText == "" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "highlightedText is required")
		return
	}

	resp, err := h.geminiClient.SynthesizeSpeech(r.Context(), req.HighlightedText, req.ContextText)
	if err != nil {
		logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context())).ErrorWithErr(err, "Failed to synthesize speech")
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to synthesize speech")
		return
	}

	contentType := resp.MIMEType
	if contentType == "" {
		contentType = "audio/wav"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp.Audio)
}

type AnnotationAnnotation struct {
	Meaning            string `json:"meaning"`
	UsageExample       string `json:"usageExample"`
	UsageTiming        string `json:"usageTiming"`
	WordBreakdown      string `json:"wordBreakdown"`
	AlternativeMeaning string `json:"alternativeMeaning"`
}

func toNuanceData(resp *gemini.AnnotationResponse) models.NuanceData {
	return models.NuanceData{
		Meaning:            resp.Meaning,
		UsageExample:       resp.UsageExample,
		UsageTiming:        resp.WhenToUse,
		WordBreakdown:      resp.WordBreakdown,
		AlternativeMeaning: resp.AlternativeMeanings,
	}
}

func summarizeNuance(nuance models.NuanceData) string {
	if len(nuance.Meaning) > 100 {
		return nuance.Meaning[:100] + "..."
	}
	return nuance.Meaning
}

func (h *AIHandlers) createAnalyzePrompt(targetLanguage, textToAnalyze, context string) string {
	languageName := map[string]string{
		"ID": "Indonesian",
		"JP": "Japanese",
		"EN": "English",
	}[targetLanguage]

	return fmt.Sprintf(`You are helping a language learner understand Japanese text.

Text to analyze: %s
Context: %s
Target language for explanation: %s

Provide a detailed annotation in JSON format with these exact fields:
- meaning: Direct translation and explanation in %s
- usage_example: Example sentence in a professional context
- usage_timing: When and in what situation this phrase is used
- word_breakdown: Breakdown of each word with translations
- alternative_meaning: Alternative meanings if any

Return only valid JSON, no markdown formatting.`, textToAnalyze, context, languageName, languageName)
}

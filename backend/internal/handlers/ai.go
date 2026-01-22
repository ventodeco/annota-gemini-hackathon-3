package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gemini-hackathon/app/internal/gemini"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/storage"
)

type AIHandlers struct {
	db           storage.DB
	geminiClient gemini.Client
}

func NewAIHandlers(db storage.DB, geminiClient gemini.Client) *AIHandlers {
	return &AIHandlers{
		db:           db,
		geminiClient: geminiClient,
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

func (h *AIHandlers) AnalyzeAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TextToAnalyze == "" {
		http.Error(w, "textToAnalyze is required", http.StatusBadRequest)
		return
	}

	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil {
		log.Printf("Failed to get user: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	targetLanguage := user.PreferredLanguage
	if targetLanguage == "" {
		targetLanguage = "ID"
	}

	resp, err := h.geminiClient.Annotate(r.Context(), req.Context, req.TextToAnalyze)
	if err != nil {
		log.Printf("Failed to generate annotation: %v", err)
		http.Error(w, "Failed to analyze text", http.StatusInternalServerError)
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TextToAnalyze == "" {
		http.Error(w, "textToAnalyze is required", http.StatusBadRequest)
		return
	}

	resp, err := h.geminiClient.Annotate(r.Context(), req.Context, req.TextToAnalyze)
	if err != nil {
		log.Printf("Failed to generate annotation with language: %v", err)
		http.Error(w, "Failed to analyze text", http.StatusInternalServerError)
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

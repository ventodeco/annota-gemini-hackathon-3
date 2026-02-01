package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/gemini-hackathon/app/internal/knowledge"
	"google.golang.org/genai"
)

type Client interface {
	OCR(ctx context.Context, imageData []byte, mimeType string) (*OCRResponse, error)
	Annotate(ctx context.Context, ocrText string, selectedText string) (*AnnotationResponse, error)
	AnnotateWithKnowledge(ctx context.Context, ocrText string, selectedText string, entries []knowledge.Entry) (*AnnotationResponse, error)
}

type client struct {
	genaiClient *genai.Client
	modelName   string
	initErr     error
}

func NewClient(apiKey string) Client {
	ctx := context.Background()
	genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return &client{
			genaiClient: nil,
			modelName:   "gemini-2.5-flash",
			initErr:     err,
		}
	}

	return &client{
		genaiClient: genaiClient,
		modelName:   "gemini-2.5-flash",
		initErr:     nil,
	}
}

type OCRResponse struct {
	RawText        string
	StructuredJSON string
	Language       string
}

type AnnotationResponse struct {
	Meaning             string `json:"meaning"`
	UsageExample        string `json:"usage_example"`
	WhenToUse           string `json:"when_to_use"`
	WordBreakdown       string `json:"word_breakdown"`
	AlternativeMeanings string `json:"alternative_meanings"`
}

func (c *client) OCR(ctx context.Context, imageData []byte, mimeType string) (*OCRResponse, error) {
	if c.genaiClient == nil {
		if c.initErr != nil {
			return nil, fmt.Errorf("gemini client not initialized: %w", c.initErr)
		}
		return nil, fmt.Errorf("gemini client not initialized: check API key")
	}

	prompt := "Extract all Japanese text from this image. Return ONLY a JSON object with keys 'raw_text' (the extracted text) and 'language' (detected language code 'JP' for japan). Preserve line breaks and formatting. Do not include markdown, code fences, or any extra text."

	parts := []*genai.Part{
		{Text: prompt},
		{
			InlineData: &genai.Blob{
				Data:     imageData,
				MIMEType: mimeType,
			},
		},
	}

	cfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"raw_text": {Type: genai.TypeString},
				"language": {Type: genai.TypeString},
			},
			Required:         []string{"raw_text", "language"},
			PropertyOrdering: []string{"raw_text", "language"},
		},
	}

	var result *genai.GenerateContentResponse
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		result, err = c.genaiClient.Models.GenerateContent(
			ctx,
			c.modelName,
			[]*genai.Content{{Parts: parts}},
			cfg,
		)
		if err == nil {
			break
		}
		if !isOverloadedError(err) || attempt == 2 {
			return nil, fmt.Errorf("failed to generate OCR content: %w", err)
		}

		// Backoff (with small jitter) before retrying overloaded/UNAVAILABLE errors.
		backoff := time.Duration(500*(1<<attempt)) * time.Millisecond
		jitter := time.Duration(rand.Intn(250)) * time.Millisecond
		if !sleepWithContext(ctx, backoff+jitter) {
			return nil, fmt.Errorf("failed to generate OCR content: %w", err)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to generate OCR content: %w", err)
	}

	text := result.Text()
	if text == "" {
		return nil, fmt.Errorf("empty response from API")
	}

	var structured struct {
		RawText  string `json:"raw_text"`
		Language string `json:"language"`
	}

	if err := json.Unmarshal([]byte(text), &structured); err != nil {
		normalized := normalizeJSONCandidate(text)
		if normalized != text {
			if err2 := json.Unmarshal([]byte(normalized), &structured); err2 == nil {
				structuredJSON, _ := json.Marshal(structured)
				return &OCRResponse{
					RawText:        structured.RawText,
					StructuredJSON: string(structuredJSON),
					Language:       structured.Language,
				}, nil
			}
		}
		return &OCRResponse{
			RawText:        text,
			StructuredJSON: "",
			Language:       "ja",
		}, nil
	}

	structuredJSON, _ := json.Marshal(structured)

	return &OCRResponse{
		RawText:        structured.RawText,
		StructuredJSON: string(structuredJSON),
		Language:       structured.Language,
	}, nil
}

func (c *client) Annotate(ctx context.Context, ocrText string, selectedText string) (*AnnotationResponse, error) {
	if c.genaiClient == nil {
		if c.initErr != nil {
			return nil, fmt.Errorf("gemini client not initialized: %w", c.initErr)
		}
		return nil, fmt.Errorf("gemini client not initialized: check API key")
	}

	prompt := fmt.Sprintf(`You are helping a Japanese language learner understand text in a professional/work context.

Full OCR text:
%s

Selected text to annotate:
%s

Provide a detailed annotation in JSON format with these exact fields:
- meaning: Direct translation of the selected text
- usage_example: Example sentence showing how to use this in a professional/work context
- when_to_use: When and in what situation this phrase is used
- word_breakdown: Explanation of each word/component in the selected text
- alternative_meanings: Alternative meanings in different fields or contexts

Return only valid JSON, no markdown formatting.`, ocrText, selectedText)

	cfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"meaning":              {Type: genai.TypeString},
				"usage_example":        {Type: genai.TypeString},
				"when_to_use":          {Type: genai.TypeString},
				"word_breakdown":       {Type: genai.TypeString},
				"alternative_meanings": {Type: genai.TypeString},
			},
			Required: []string{
				"meaning",
				"usage_example",
				"when_to_use",
				"word_breakdown",
				"alternative_meanings",
			},
			PropertyOrdering: []string{
				"meaning",
				"usage_example",
				"when_to_use",
				"word_breakdown",
				"alternative_meanings",
			},
		},
	}

	var result *genai.GenerateContentResponse
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		result, err = c.genaiClient.Models.GenerateContent(
			ctx,
			c.modelName,
			genai.Text(prompt),
			cfg,
		)
		if err == nil {
			break
		}
		if !isOverloadedError(err) || attempt == 2 {
			return nil, fmt.Errorf("failed to generate annotation: %w", err)
		}

		backoff := time.Duration(500*(1<<attempt)) * time.Millisecond
		jitter := time.Duration(rand.Intn(250)) * time.Millisecond
		if !sleepWithContext(ctx, backoff+jitter) {
			return nil, fmt.Errorf("failed to generate annotation: %w", err)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to generate annotation: %w", err)
	}

	text := result.Text()
	if text == "" {
		return nil, fmt.Errorf("empty response from API")
	}

	var annotation AnnotationResponse
	if err := json.Unmarshal([]byte(text), &annotation); err != nil {
		normalized := normalizeJSONCandidate(text)
		if normalized != text {
			if err2 := json.Unmarshal([]byte(normalized), &annotation); err2 == nil {
				return &annotation, nil
			}
		}
		return nil, fmt.Errorf("failed to parse annotation JSON: %w", err)
	}

	return &annotation, nil
}

func (c *client) AnnotateWithKnowledge(ctx context.Context, ocrText string, selectedText string, entries []knowledge.Entry) (*AnnotationResponse, error) {
	if c.genaiClient == nil {
		if c.initErr != nil {
			return nil, fmt.Errorf("gemini client not initialized: %w", c.initErr)
		}
		return nil, fmt.Errorf("gemini client not initialized: check API key")
	}

	prompt := buildEnhancedPrompt(ocrText, selectedText, entries)

	cfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"meaning":              {Type: genai.TypeString},
				"usage_example":        {Type: genai.TypeString},
				"when_to_use":          {Type: genai.TypeString},
				"word_breakdown":       {Type: genai.TypeString},
				"alternative_meanings": {Type: genai.TypeString},
			},
			Required: []string{
				"meaning",
				"usage_example",
				"when_to_use",
				"word_breakdown",
				"alternative_meanings",
			},
			PropertyOrdering: []string{
				"meaning",
				"usage_example",
				"when_to_use",
				"word_breakdown",
				"alternative_meanings",
			},
		},
	}

	var result *genai.GenerateContentResponse
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		result, err = c.genaiClient.Models.GenerateContent(
			ctx,
			c.modelName,
			genai.Text(prompt),
			cfg,
		)
		if err == nil {
			break
		}
		if !isOverloadedError(err) || attempt == 2 {
			return nil, fmt.Errorf("failed to generate annotation: %w", err)
		}

		backoff := time.Duration(500*(1<<attempt)) * time.Millisecond
		jitter := time.Duration(rand.Intn(250)) * time.Millisecond
		if !sleepWithContext(ctx, backoff+jitter) {
			return nil, fmt.Errorf("failed to generate annotation: %w", err)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to generate annotation: %w", err)
	}

	text := result.Text()
	if text == "" {
		return nil, fmt.Errorf("empty response from API")
	}

	var annotation AnnotationResponse
	if err := json.Unmarshal([]byte(text), &annotation); err != nil {
		normalized := normalizeJSONCandidate(text)
		if normalized != text {
			if err2 := json.Unmarshal([]byte(normalized), &annotation); err2 == nil {
				return &annotation, nil
			}
		}
		return nil, fmt.Errorf("failed to parse annotation JSON: %w", err)
	}

	return &annotation, nil
}

// buildEnhancedPrompt creates a prompt that includes reference knowledge from CSV.
func buildEnhancedPrompt(ocrText string, selectedText string, entries []knowledge.Entry) string {
	var sb strings.Builder

	sb.WriteString("You are helping a Japanese language learner understand text in a professional/work context.\n\n")

	// Add reference knowledge if available
	if len(entries) > 0 {
		sb.WriteString("## Reference Knowledge:\n")
		for _, entry := range entries {
			sb.WriteString(fmt.Sprintf("Term: %s (%s)\n", entry.Kosakata, entry.Kana))
			sb.WriteString(fmt.Sprintf("Reading: %s\n", entry.CaraBaca))
			sb.WriteString(fmt.Sprintf("Meaning: %s\n", entry.Arti))
			if entry.Deskripsi != "" {
				sb.WriteString(fmt.Sprintf("Description: %s\n", entry.Deskripsi))
			}
			if len(entry.BidangPekerjaan) > 0 {
				sb.WriteString(fmt.Sprintf("Fields: %s\n", strings.Join(entry.BidangPekerjaan, ", ")))
			}
			if len(entry.Industri) > 0 {
				sb.WriteString(fmt.Sprintf("Industry: %s\n", strings.Join(entry.Industri, ", ")))
			}
			if entry.Konteks != "" {
				sb.WriteString(fmt.Sprintf("Context: %s\n", entry.Konteks))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("## Full OCR text:\n")
	sb.WriteString(ocrText)
	sb.WriteString("\n\n")

	sb.WriteString("## Selected text to annotate:\n")
	sb.WriteString(selectedText)
	sb.WriteString("\n\n")

	if len(entries) > 0 {
		sb.WriteString("Using the reference knowledge above, provide a detailed annotation in JSON format with these exact fields:\n")
	} else {
		sb.WriteString("Provide a detailed annotation in JSON format with these exact fields:\n")
	}
	sb.WriteString("- meaning: Direct translation and explanation\n")
	sb.WriteString("- usage_example: Example sentence showing how to use this in a professional/work context\n")
	sb.WriteString("- when_to_use: When and in what situation this phrase is used\n")
	sb.WriteString("- word_breakdown: Explanation of each word/component in the selected text\n")
	sb.WriteString("- alternative_meanings: Alternative meanings in different fields or contexts\n\n")
	sb.WriteString("Return only valid JSON, no markdown formatting.")

	return sb.String()
}

func isOverloadedError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "503") || strings.Contains(msg, "unavailable") || strings.Contains(msg, "overloaded")
}

func sleepWithContext(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func normalizeJSONCandidate(s string) string {
	trimmed := strings.TrimSpace(s)

	// Strip common fenced JSON blocks: ```json\n{...}\n```
	if strings.HasPrefix(trimmed, "```") {
		if idx := strings.Index(trimmed, "\n"); idx != -1 {
			trimmed = strings.TrimSpace(trimmed[idx+1:])
		}
		trimmed = strings.TrimSuffix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
	}

	// If there's extra text, try to extract the outermost JSON object.
	start := strings.IndexByte(trimmed, '{')
	end := strings.LastIndexByte(trimmed, '}')
	if start != -1 && end != -1 && end > start {
		return strings.TrimSpace(trimmed[start : end+1])
	}
	return strings.TrimSpace(s)
}

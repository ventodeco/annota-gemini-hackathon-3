package gemini

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/genai"
)

type Client interface {
	OCR(ctx context.Context, imageData []byte, mimeType string) (*OCRResponse, error)
	Annotate(ctx context.Context, ocrText string, selectedText string) (*AnnotationResponse, error)
}

type client struct {
	genaiClient *genai.Client
	modelName   string
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
		}
	}

	return &client{
		genaiClient: genaiClient,
		modelName:   "gemini-2.5-flash",
	}
}

type OCRResponse struct {
	RawText        string
	StructuredJSON string
	Language       string
}

type AnnotationResponse struct {
	Meaning            string
	UsageExample       string
	WhenToUse          string
	WordBreakdown      string
	AlternativeMeanings string
}

func (c *client) OCR(ctx context.Context, imageData []byte, mimeType string) (*OCRResponse, error) {
	if c.genaiClient == nil {
		return nil, fmt.Errorf("gemini client not initialized: check API key")
	}

	prompt := "Extract all Japanese text from this image. Return a JSON object with 'raw_text' (the extracted text) and 'language' (detected language code). Preserve line breaks and formatting."

	parts := []*genai.Part{
		{Text: prompt},
		{
			InlineData: &genai.Blob{
				Data:     imageData,
				MIMEType: mimeType,
			},
		},
	}

	result, err := c.genaiClient.Models.GenerateContent(
		ctx,
		c.modelName,
		[]*genai.Content{{Parts: parts}},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate OCR content: %w", err)
	}

	text := result.Text()
	if text == "" {
		return nil, fmt.Errorf("empty response from API")
	}

	var structured struct {
		RawText string `json:"raw_text"`
		Language string `json:"language"`
	}

	if err := json.Unmarshal([]byte(text), &structured); err != nil {
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

	result, err := c.genaiClient.Models.GenerateContent(
		ctx,
		c.modelName,
		genai.Text(prompt),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate annotation: %w", err)
	}

	text := result.Text()
	if text == "" {
		return nil, fmt.Errorf("empty response from API")
	}

	var annotation AnnotationResponse
	if err := json.Unmarshal([]byte(text), &annotation); err != nil {
		return nil, fmt.Errorf("failed to parse annotation JSON: %w", err)
	}

	return &annotation, nil
}

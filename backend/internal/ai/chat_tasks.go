package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/gemini-hackathon/app/internal/knowledge"
)

type chatTasks struct{ ocr, annotation *taskGateway }

type gatewayChatResponse struct {
	ID      string          `json:"id"`
	Model   string          `json:"model"`
	Error   json.RawMessage `json:"error"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content string `json:"content"`
			Refusal string `json:"refusal"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     *int64       `json:"prompt_tokens"`
		CompletionTokens *int64       `json:"completion_tokens"`
		TotalTokens      *int64       `json:"total_tokens"`
		Cost             *json.Number `json:"cost"`
	} `json:"usage"`
}

func (g *taskGateway) chat(ctx context.Context, messages []map[string]any, schema map[string]any, parse func(string) error) error {
	body := map[string]any{"model": g.route.Model, "messages": messages, "max_tokens": g.model.MaxOutputTokens, "stream": false}
	mode := g.model.JSONMode
	if g.task == "ocr" && g.model.OCROutput == "markdown" {
		mode = "prompt"
	}
	switch mode {
	case "json_object":
		body["response_format"] = map[string]any{"type": "json_object"}
	case "json_schema":
		body["response_format"] = map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": g.task, "strict": true, "schema": schema}}
	}
	if g.profile.Kind == "openrouter" {
		provider := map[string]any{"allow_fallbacks": false}
		if mode != "prompt" {
			provider["require_parameters"] = true
		}
		body["provider"] = provider
	}
	return g.call(ctx, "/chat/completions", body, 8<<20, func(data []byte, _ http.Header, meta *CallMetadata) error {
		var response gatewayChatResponse
		if json.Unmarshal(data, &response) != nil {
			return g.failure(ErrorKindUpstream, meta.StatusCode, "malformed chat response", nil)
		}
		// Accounting may still be reported for rejected/incomplete generations.
		meta.InputTokens = response.Usage.PromptTokens
		meta.OutputTokens = response.Usage.CompletionTokens
		meta.TotalTokens = response.Usage.TotalTokens
		meta.Cost = response.Usage.Cost
		if meta.Cost != nil && g.profile.Kind == "openrouter" {
			meta.Currency = "USD"
		}
		if meta.RequestID == "" {
			meta.RequestID = g.safeID(response.ID)
		}
		if response.Model != "" {
			if response.Model != g.route.Model && !slices.Contains(g.model.ReportedModelAliases, response.Model) {
				return g.failure(ErrorKindUpstream, meta.StatusCode, "gateway returned an unapproved model", nil)
			}
			meta.ReportedModel = response.Model
		}
		if len(response.Error) > 0 && string(response.Error) != "null" {
			var failure struct {
				Code int `json:"code"`
			}
			kind := ErrorKindUpstream
			if json.Unmarshal(response.Error, &failure) == nil {
				kind = statusKind(failure.Code)
			}
			return g.failure(kind, meta.StatusCode, "gateway reported a generation error", nil)
		}
		if len(response.Choices) == 0 {
			return g.failure(ErrorKindUpstream, meta.StatusCode, "chat response has no result", nil)
		}
		choice := response.Choices[0]
		if choice.FinishReason != "stop" || choice.Message.Refusal != "" {
			return g.failure(ErrorKindUpstream, meta.StatusCode, "generation refused or incomplete", nil)
		}
		if strings.TrimSpace(choice.Message.Content) == "" {
			return g.failure(ErrorKindUpstream, meta.StatusCode, "generation is empty", nil)
		}
		if err := parse(choice.Message.Content); err != nil {
			return g.failure(ErrorKindUpstream, meta.StatusCode, "generation does not match the task schema", nil)
		}
		return nil
	})
}

func (c *chatTasks) OCR(ctx context.Context, image []byte, mime string) (*OCRResponse, error) {
	if len(image) == 0 || !slices.Contains([]string{"image/png", "image/jpeg", "image/webp", "image/gif"}, mime) {
		return nil, c.ocr.failure(ErrorKindConfig, 0, "OCR requires an image and supported MIME type", nil)
	}
	prompt := routedOCRJSONPrompt
	if c.ocr.model.OCROutput == "markdown" {
		prompt = routedOCRMarkdownPrompt
	}
	messages := []map[string]any{{"role": "user", "content": []map[string]any{{"type": "text", "text": prompt}, {"type": "image_url", "image_url": map[string]string{"url": "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(image)}}}}}
	schema := map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"raw_text": map[string]string{"type": "string"}, "language": map[string]string{"type": "string"}}, "required": []string{"raw_text", "language"}}
	var result *OCRResponse
	err := c.ocr.chat(ctx, messages, schema, func(text string) error {
		if c.ocr.model.OCROutput == "markdown" {
			result = ocrResponseFromStructured(strings.TrimSpace(text), "ja")
			return nil
		}
		var value struct {
			RawText  string `json:"raw_text"`
			Language string `json:"language"`
		}
		object, parseErr := routedJSONObject(text)
		if parseErr != nil || json.Unmarshal(object, &value) != nil || strings.TrimSpace(value.RawText) == "" {
			return c.ocr.failure(ErrorKindUpstream, 0, "invalid OCR JSON", nil)
		}
		result = ocrResponseFromStructured(value.RawText, value.Language)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *chatTasks) Annotate(ctx context.Context, text, selected string) (*AnnotationResponse, error) {
	return c.AnnotateWithKnowledge(ctx, text, selected, nil)
}
func (c *chatTasks) AnnotateWithKnowledge(ctx context.Context, text, selected string, entries []knowledge.Entry) (*AnnotationResponse, error) {
	var result *AnnotationResponse
	err := c.annotation.chat(ctx, []map[string]any{{"role": "user", "content": buildEnhancedPrompt(text, selected, entries)}}, annotationJSONSchema(), func(raw string) error {
		object, err := routedJSONObject(raw)
		if err != nil || !validRoutedAnnotationTypes(object) {
			return fmt.Errorf("invalid annotation shape")
		}
		annotation, err := decodeAnnotationJSON(string(object))
		if err != nil {
			return err
		}
		for _, field := range []string{annotation.Translation, annotation.ContextualExplanation, annotation.UsageExample, annotation.WhenToUse, annotation.WordBreakdown, annotation.AlternativeMeanings, annotation.Pronunciation.Kana} {
			if strings.TrimSpace(field) == "" {
				return c.annotation.failure(ErrorKindUpstream, 0, "incomplete annotation", nil)
			}
		}
		result = annotation
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func annotationJSONSchema() map[string]any {
	properties := map[string]any{}
	required := []string{"translation", "contextual_explanation", "usage_example", "when_to_use", "word_breakdown", "alternative_meanings", "pronunciation"}
	for _, name := range required[:6] {
		properties[name] = map[string]string{"type": "string"}
	}
	properties["pronunciation"] = map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"kana": map[string]string{"type": "string"}, "romaji": map[string]string{"type": "string"}}, "required": []string{"kana", "romaji"}}
	return map[string]any{"type": "object", "additionalProperties": false, "properties": properties, "required": required}
}

// routedJSONObject permits JSON fences, but never extracts an object from prose or arrays.
// Legacy normalization remains unchanged for legacy provider modes.
func routedJSONObject(text string) ([]byte, error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		newline := strings.IndexByte(text, '\n')
		if newline < 0 || !strings.HasSuffix(text, "```") {
			return nil, fmt.Errorf("invalid JSON fence")
		}
		language := strings.TrimSpace(strings.TrimPrefix(text[:newline], "```"))
		if language != "" && !strings.EqualFold(language, "json") {
			return nil, fmt.Errorf("unsupported JSON fence")
		}
		text = strings.TrimSpace(strings.TrimSuffix(text[newline+1:], "```"))
	}
	data := []byte(text)
	if len(data) == 0 || data[0] != '{' || !json.Valid(data) {
		return nil, fmt.Errorf("expected one JSON object")
	}
	return data, nil
}

func validRoutedAnnotationTypes(object []byte) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(object, &fields) != nil {
		return false
	}
	for _, key := range []string{"translation", "contextual_explanation", "usage_example", "when_to_use", "alternative_meanings", "meaning", "alternative_meaning"} {
		if value, exists := fields[key]; exists {
			value = bytes.TrimSpace(value)
			if len(value) == 0 || value[0] != '"' {
				return false
			}
		}
	}
	breakdown := bytes.TrimSpace(fields["word_breakdown"])
	if len(breakdown) == 0 {
		return false
	}
	if breakdown[0] != '"' {
		var words map[string]json.RawMessage
		if breakdown[0] != '{' || json.Unmarshal(breakdown, &words) != nil || len(words) == 0 {
			return false
		}
	}
	var pronunciation map[string]json.RawMessage
	if json.Unmarshal(fields["pronunciation"], &pronunciation) != nil || len(pronunciation) == 0 {
		return false
	}
	for _, key := range []string{"kana", "romaji"} {
		if value, exists := pronunciation[key]; exists {
			value = bytes.TrimSpace(value)
			if len(value) == 0 || value[0] != '"' {
				return false
			}
		}
	}
	return true
}

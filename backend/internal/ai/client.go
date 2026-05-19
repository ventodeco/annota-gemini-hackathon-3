package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gemini-hackathon/app/internal/knowledge"
	"google.golang.org/genai"
)

const (
	ProviderGemini  = "gemini"
	ProviderMiniMax = "minimax"
)

type Client interface {
	OCR(ctx context.Context, imageData []byte, mimeType string) (*OCRResponse, error)
	Annotate(ctx context.Context, ocrText string, selectedText string) (*AnnotationResponse, error)
	AnnotateWithKnowledge(ctx context.Context, ocrText string, selectedText string, entries []knowledge.Entry) (*AnnotationResponse, error)
	SynthesizeSpeech(ctx context.Context, highlightedText string, contextText string) (*SpeechResponse, error)
}

type ClientConfig struct {
	AIProvider              string
	GeminiAPIKey            string
	OpenRouterAPIKey        string
	OpenRouterBaseURL       string
	OpenRouterOCRModel      string
	MiniMaxAPIKey           string
	MiniMaxAnthropicBaseURL string
	MiniMaxTextModel        string
	MiniMaxTTSBaseURL       string
	MiniMaxTTSModel         string
	MiniMaxTTSVoiceID       string
	HTTPClient              *http.Client
}

type client struct {
	cfg        ClientConfig
	httpClient *http.Client
}

type OCRResponse struct {
	RawText        string
	StructuredJSON string
	Language       string
}

type AnnotationResponse struct {
	Translation           string        `json:"translation"`
	ContextualExplanation string        `json:"contextual_explanation"`
	UsageExample          string        `json:"usage_example"`
	WhenToUse             string        `json:"when_to_use"`
	WordBreakdown         string        `json:"word_breakdown"`
	AlternativeMeanings   string        `json:"alternative_meanings"`
	Pronunciation         Pronunciation `json:"pronunciation"`
	Meaning               string        `json:"meaning"`
	AlternativeMeaning    string        `json:"alternative_meaning"`
}

type Pronunciation struct {
	Kana   string `json:"kana"`
	Romaji string `json:"romaji,omitempty"`
}

type SpeechResponse struct {
	Audio    []byte
	MIMEType string
}

type ErrorKind string

const (
	ErrorKindConfig    ErrorKind = "config"
	ErrorKindAuth      ErrorKind = "auth"
	ErrorKindRateLimit ErrorKind = "rate_limit"
	ErrorKindQuota     ErrorKind = "quota"
	ErrorKindUpstream  ErrorKind = "upstream"
)

type ProviderError struct {
	Provider   string
	Kind       ErrorKind
	StatusCode int
	Message    string
}

func (e *ProviderError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("%s %s error (%d): %s", e.Provider, e.Kind, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("%s %s error: %s", e.Provider, e.Kind, e.Message)
}

func NewClient(cfg ClientConfig) Client {
	applyDefaults(&cfg)
	if cfg.normalizedProvider() == ProviderGemini {
		return newGeminiClient(cfg)
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &client{cfg: cfg, httpClient: httpClient}
}

func (cfg ClientConfig) normalizedProvider() string {
	provider := strings.ToLower(strings.TrimSpace(cfg.AIProvider))
	if provider == "" {
		return ProviderMiniMax
	}
	return provider
}

func applyDefaults(cfg *ClientConfig) {
	if cfg.OpenRouterBaseURL == "" {
		cfg.OpenRouterBaseURL = "https://openrouter.ai/api/v1"
	}
	if cfg.OpenRouterOCRModel == "" {
		cfg.OpenRouterOCRModel = "baidu/qianfan-ocr-fast:free"
	}
	if cfg.MiniMaxAnthropicBaseURL == "" {
		cfg.MiniMaxAnthropicBaseURL = "https://api.minimax.io/anthropic/v1"
	}
	if cfg.MiniMaxTextModel == "" {
		cfg.MiniMaxTextModel = "MiniMax-M2.7"
	}
	if cfg.MiniMaxTTSBaseURL == "" {
		cfg.MiniMaxTTSBaseURL = "https://api-uw.minimax.io/v1"
	}
	if cfg.MiniMaxTTSModel == "" {
		cfg.MiniMaxTTSModel = "speech-2.8-hd"
	}
	if cfg.MiniMaxTTSVoiceID == "" {
		cfg.MiniMaxTTSVoiceID = "Japanese_Whisper_Belle"
	}
}

type geminiClient struct {
	apiKey      string
	genaiClient *genai.Client
	modelName   string
	initErr     error
}

func newGeminiClient(cfg ClientConfig) Client {
	client := &geminiClient{
		apiKey:    cfg.GeminiAPIKey,
		modelName: "gemini-2.5-flash",
	}
	if cfg.GeminiAPIKey == "" {
		return client
	}

	genaiClient, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		client.initErr = err
		return client
	}
	client.genaiClient = genaiClient
	return client
}

func (c *geminiClient) ensureReady() error {
	if c.apiKey == "" {
		return &ProviderError{Provider: ProviderGemini, Kind: ErrorKindConfig, Message: "GEMINI_API_KEY or GOOGLE_API_KEY is required"}
	}
	if c.genaiClient == nil {
		message := "Gemini client is not initialized"
		if c.initErr != nil {
			message = c.initErr.Error()
		}
		return &ProviderError{Provider: ProviderGemini, Kind: ErrorKindConfig, Message: message}
	}
	return nil
}

func (c *geminiClient) OCR(ctx context.Context, imageData []byte, mimeType string) (*OCRResponse, error) {
	if err := c.ensureReady(); err != nil {
		return nil, err
	}

	parts := []*genai.Part{
		{Text: "Extract all Japanese text from this image. Return ONLY a JSON object with keys raw_text and language. Use language code ja for Japanese. Preserve line breaks and formatting."},
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

	result, err := c.generateWithRetry(ctx, c.modelName, []*genai.Content{{Parts: parts}}, cfg)
	if err != nil {
		return nil, wrapGeminiError("failed to generate OCR content", err)
	}

	text := strings.TrimSpace(result.Text())
	if text == "" {
		return nil, fmt.Errorf("empty response from Gemini OCR")
	}

	var structured struct {
		RawText  string `json:"raw_text"`
		Language string `json:"language"`
	}
	if err := json.Unmarshal([]byte(text), &structured); err != nil {
		normalized := normalizeJSONCandidate(text)
		if normalized != text {
			if err2 := json.Unmarshal([]byte(normalized), &structured); err2 == nil {
				return ocrResponseFromStructured(structured.RawText, structured.Language), nil
			}
		}
		return &OCRResponse{RawText: text, StructuredJSON: "", Language: "ja"}, nil
	}
	return ocrResponseFromStructured(structured.RawText, structured.Language), nil
}

func (c *geminiClient) Annotate(ctx context.Context, ocrText string, selectedText string) (*AnnotationResponse, error) {
	return c.AnnotateWithKnowledge(ctx, ocrText, selectedText, nil)
}

func (c *geminiClient) AnnotateWithKnowledge(ctx context.Context, ocrText string, selectedText string, entries []knowledge.Entry) (*AnnotationResponse, error) {
	if err := c.ensureReady(); err != nil {
		return nil, err
	}

	cfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema:   annotationResponseSchema(),
	}
	result, err := c.generateWithRetry(ctx, c.modelName, genai.Text(buildEnhancedPrompt(ocrText, selectedText, entries)), cfg)
	if err != nil {
		return nil, wrapGeminiError("failed to generate annotation", err)
	}

	text := strings.TrimSpace(result.Text())
	if text == "" {
		return nil, fmt.Errorf("empty response from Gemini annotation")
	}

	annotation, err := decodeAnnotationJSON(text)
	if err != nil {
		return nil, fmt.Errorf("failed to parse annotation JSON: %w", err)
	}
	return annotation, nil
}

func (c *geminiClient) SynthesizeSpeech(ctx context.Context, highlightedText string, contextText string) (*SpeechResponse, error) {
	if err := c.ensureReady(); err != nil {
		return nil, err
	}

	cfg := &genai.GenerateContentConfig{
		ResponseModalities: []string{"AUDIO"},
		SpeechConfig: &genai.SpeechConfig{
			VoiceConfig: &genai.VoiceConfig{
				PrebuiltVoiceConfig: &genai.PrebuiltVoiceConfig{VoiceName: "Kore"},
			},
			LanguageCode: "ja-JP",
		},
	}

	result, err := c.genaiClient.Models.GenerateContent(ctx, "gemini-2.5-flash-preview-tts", genai.Text(buildSpeechPrompt(highlightedText, contextText)), cfg)
	if err != nil {
		return nil, wrapGeminiError("failed to synthesize speech", err)
	}

	for _, candidate := range result.Candidates {
		if candidate == nil || candidate.Content == nil {
			continue
		}
		for _, part := range candidate.Content.Parts {
			if part == nil || part.InlineData == nil || len(part.InlineData.Data) == 0 {
				continue
			}
			mimeType := strings.ToLower(strings.TrimSpace(part.InlineData.MIMEType))
			audio := part.InlineData.Data
			if mimeType == "" || strings.Contains(mimeType, "pcm") {
				audio = wrapPCMAsWAV(audio, 24000, 1, 16)
				mimeType = "audio/wav"
			}
			return &SpeechResponse{Audio: audio, MIMEType: mimeType}, nil
		}
	}

	return nil, fmt.Errorf("no audio data in Gemini response")
}

func (c *geminiClient) generateWithRetry(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	var result *genai.GenerateContentResponse
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		result, err = c.genaiClient.Models.GenerateContent(ctx, model, contents, cfg)
		if err == nil {
			return result, nil
		}
		if !isOverloadedError(err) || attempt == 2 {
			return nil, err
		}
		if !sleepWithContext(ctx, retryBackoff(attempt)) {
			return nil, ctx.Err()
		}
	}
	return nil, err
}

func (c *client) OCR(ctx context.Context, imageData []byte, mimeType string) (*OCRResponse, error) {
	if c.cfg.OpenRouterAPIKey == "" {
		return nil, &ProviderError{Provider: "openrouter", Kind: ErrorKindConfig, Message: "OPENROUTER_API_KEY is required"}
	}

	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(imageData))
	requestBody := map[string]any{
		"model": c.cfg.OpenRouterOCRModel,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "text",
						"text": "Extract all readable text from this image. Return only JSON with keys raw_text and language. Use language code ja for Japanese.",
					},
					{
						"type": "image_url",
						"image_url": map[string]string{
							"url": dataURL,
						},
					},
				},
			},
		},
	}

	var response openRouterChatResponse
	if err := c.postJSON(ctx, "openrouter", c.cfg.OpenRouterBaseURL, "/chat/completions", c.cfg.OpenRouterAPIKey, requestBody, &response); err != nil {
		return nil, err
	}
	text := strings.TrimSpace(response.firstText())
	if text == "" {
		return nil, fmt.Errorf("empty response from OpenRouter OCR")
	}

	var structured struct {
		RawText  string `json:"raw_text"`
		Language string `json:"language"`
	}
	if err := json.Unmarshal([]byte(text), &structured); err != nil {
		normalized := normalizeJSONCandidate(text)
		if normalized != text {
			if err2 := json.Unmarshal([]byte(normalized), &structured); err2 == nil {
				return ocrResponseFromStructured(structured.RawText, structured.Language), nil
			}
		}
		return &OCRResponse{RawText: text, StructuredJSON: "", Language: "ja"}, nil
	}
	return ocrResponseFromStructured(structured.RawText, structured.Language), nil
}

func ocrResponseFromStructured(rawText, language string) *OCRResponse {
	if language == "" {
		language = "ja"
	}
	structuredJSON, _ := json.Marshal(struct {
		RawText  string `json:"raw_text"`
		Language string `json:"language"`
	}{RawText: rawText, Language: language})
	return &OCRResponse{RawText: rawText, StructuredJSON: string(structuredJSON), Language: language}
}

func (c *client) Annotate(ctx context.Context, ocrText string, selectedText string) (*AnnotationResponse, error) {
	return c.AnnotateWithKnowledge(ctx, ocrText, selectedText, nil)
}

func (c *client) AnnotateWithKnowledge(ctx context.Context, ocrText string, selectedText string, entries []knowledge.Entry) (*AnnotationResponse, error) {
	if c.cfg.MiniMaxAPIKey == "" {
		return nil, &ProviderError{Provider: "minimax", Kind: ErrorKindConfig, Message: "MINIMAX_API_KEY is required"}
	}

	requestBody := map[string]any{
		"model":      c.cfg.MiniMaxTextModel,
		"max_tokens": 1200,
		"system":     "You help Japanese language learners. Return only valid JSON matching the requested schema.",
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]string{
					{
						"type": "text",
						"text": buildEnhancedPrompt(ocrText, selectedText, entries),
					},
				},
			},
		},
	}

	var response anthropicMessageResponse
	if err := c.postJSON(ctx, "minimax", c.cfg.MiniMaxAnthropicBaseURL, "/messages", c.cfg.MiniMaxAPIKey, requestBody, &response); err != nil {
		return nil, err
	}
	text := strings.TrimSpace(response.firstText())
	if text == "" {
		return nil, fmt.Errorf("empty response from MiniMax annotation")
	}

	annotation, err := decodeAnnotationJSON(text)
	if err != nil {
		return nil, fmt.Errorf("failed to parse annotation JSON: %w", err)
	}
	return annotation, nil
}

func (c *client) SynthesizeSpeech(ctx context.Context, highlightedText string, _ string) (*SpeechResponse, error) {
	if c.cfg.MiniMaxAPIKey == "" {
		return nil, &ProviderError{Provider: "minimax", Kind: ErrorKindConfig, Message: "MINIMAX_API_KEY is required"}
	}

	requestBody := map[string]any{
		"model":          c.cfg.MiniMaxTTSModel,
		"text":           highlightedText,
		"stream":         false,
		"language_boost": "Japanese",
		"output_format":  "hex",
		"voice_setting": map[string]any{
			"voice_id": c.cfg.MiniMaxTTSVoiceID,
			"speed":    1,
			"vol":      1,
			"pitch":    0,
		},
		"audio_setting": map[string]any{
			"sample_rate": 32000,
			"bitrate":     128000,
			"format":      "mp3",
			"channel":     1,
		},
	}

	var response minimaxTTSResponse
	if err := c.postJSON(ctx, "minimax", c.cfg.MiniMaxTTSBaseURL, "/t2a_v2", c.cfg.MiniMaxAPIKey, requestBody, &response); err != nil {
		return nil, err
	}
	if response.BaseResp.StatusCode != 0 {
		return nil, &ProviderError{
			Provider: "minimax",
			Kind:     classifyMiniMaxStatus(response.BaseResp.StatusCode, response.BaseResp.StatusMsg),
			Message:  response.BaseResp.StatusMsg,
		}
	}
	audio, err := hex.DecodeString(response.Data.Audio)
	if err != nil {
		return nil, fmt.Errorf("failed to decode MiniMax audio: %w", err)
	}
	if len(audio) == 0 {
		return nil, fmt.Errorf("empty audio from MiniMax TTS")
	}
	return &SpeechResponse{Audio: audio, MIMEType: mimeTypeForAudioFormat(response.ExtraInfo.AudioFormat)}, nil
}

func (c *client) postJSON(ctx context.Context, provider, baseURL, path, apiKey string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to encode %s request: %w", provider, err)
	}

	var responseBody []byte
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint(baseURL, path), bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("failed to build %s request: %w", provider, err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")
		if provider == "minimax" && path == "/messages" {
			req.Header.Set("anthropic-version", "2023-06-01")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if attempt == 2 {
				return fmt.Errorf("failed to call %s: %w", provider, err)
			}
			if !sleepWithContext(ctx, retryBackoff(attempt)) {
				return fmt.Errorf("failed to call %s: %w", provider, ctx.Err())
			}
			continue
		}

		responseBody, err = io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read %s response: %w", provider, err)
		}
		if closeErr != nil {
			return fmt.Errorf("failed to close %s response: %w", provider, closeErr)
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			break
		}
		providerErr := classifyHTTPError(provider, resp.StatusCode, string(responseBody))
		if !isRetryableStatus(resp.StatusCode) || attempt == 2 {
			return providerErr
		}
		if !sleepWithContext(ctx, retryBackoff(attempt)) {
			return providerErr
		}
	}

	if err := json.Unmarshal(responseBody, out); err != nil {
		return fmt.Errorf("failed to parse %s response: %w", provider, err)
	}
	return nil
}

func endpoint(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + path
}

func retryBackoff(attempt int) time.Duration {
	backoff := time.Duration(500*(1<<attempt)) * time.Millisecond
	jitter := time.Duration(rand.Intn(250)) * time.Millisecond
	return backoff + jitter
}

func isRetryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func classifyHTTPError(provider string, status int, body string) error {
	kind := ErrorKindUpstream
	bodyLower := strings.ToLower(body)
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		kind = ErrorKindAuth
	case status == http.StatusTooManyRequests:
		kind = ErrorKindRateLimit
	case strings.Contains(bodyLower, "quota") || strings.Contains(bodyLower, "insufficient") || strings.Contains(bodyLower, "resource_exhausted"):
		kind = ErrorKindQuota
	}
	return &ProviderError{Provider: provider, Kind: kind, StatusCode: status, Message: strings.TrimSpace(body)}
}

func classifyMiniMaxStatus(status int, message string) ErrorKind {
	switch status {
	case 1002, 1039:
		return ErrorKindRateLimit
	case 1004:
		return ErrorKindAuth
	default:
		msg := strings.ToLower(message)
		if strings.Contains(msg, "not support model") || strings.Contains(msg, "not supported model") {
			return ErrorKindConfig
		}
		if strings.Contains(msg, "quota") || strings.Contains(msg, "balance") || strings.Contains(msg, "insufficient") {
			return ErrorKindQuota
		}
		return ErrorKindUpstream
	}
}

func wrapGeminiError(message string, err error) error {
	if err == nil {
		return nil
	}
	kind := ErrorKindUpstream
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "api key") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "permission"):
		kind = ErrorKindAuth
	case strings.Contains(msg, "quota") || strings.Contains(msg, "resource_exhausted") || strings.Contains(msg, "insufficient"):
		kind = ErrorKindQuota
	case strings.Contains(msg, "rate limit"):
		kind = ErrorKindRateLimit
	}
	return &ProviderError{Provider: ProviderGemini, Kind: kind, Message: fmt.Sprintf("%s: %v", message, err)}
}

func isOverloadedError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "503") || strings.Contains(msg, "unavailable") || strings.Contains(msg, "overloaded")
}

func mimeTypeForAudioFormat(format string) string {
	switch strings.ToLower(format) {
	case "mp3":
		return "audio/mpeg"
	case "wav":
		return "audio/wav"
	case "flac":
		return "audio/flac"
	case "pcm":
		return "application/octet-stream"
	default:
		return "audio/mpeg"
	}
}

func annotationResponseSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"translation":            {Type: genai.TypeString},
			"contextual_explanation": {Type: genai.TypeString},
			"usage_example":          {Type: genai.TypeString},
			"when_to_use":            {Type: genai.TypeString},
			"word_breakdown":         {Type: genai.TypeString},
			"alternative_meanings":   {Type: genai.TypeString},
			"pronunciation": {
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"kana":   {Type: genai.TypeString},
					"romaji": {Type: genai.TypeString},
				},
				Required:         []string{"kana"},
				PropertyOrdering: []string{"kana", "romaji"},
			},
		},
		Required: []string{
			"translation",
			"contextual_explanation",
			"usage_example",
			"when_to_use",
			"word_breakdown",
			"alternative_meanings",
			"pronunciation",
		},
		PropertyOrdering: []string{
			"translation",
			"contextual_explanation",
			"usage_example",
			"when_to_use",
			"word_breakdown",
			"alternative_meanings",
			"pronunciation",
		},
	}
}

type openRouterChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (r openRouterChatResponse) firstText() string {
	if len(r.Choices) == 0 {
		return ""
	}
	return r.Choices[0].Message.Content
}

type anthropicMessageResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func (r anthropicMessageResponse) firstText() string {
	for _, block := range r.Content {
		if block.Type == "text" && block.Text != "" {
			return block.Text
		}
	}
	return ""
}

type minimaxTTSResponse struct {
	Data struct {
		Audio  string `json:"audio"`
		Status int    `json:"status"`
	} `json:"data"`
	ExtraInfo struct {
		AudioFormat string `json:"audio_format"`
	} `json:"extra_info"`
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

func buildEnhancedPrompt(ocrText string, selectedText string, entries []knowledge.Entry) string {
	var sb strings.Builder

	sb.WriteString("You are helping a Japanese language learner understand text in a professional/work context. Respond in English.\n\n")
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
		sb.WriteString("Using the reference knowledge above, provide a detailed annotation in English JSON format with these exact fields:\n")
	} else {
		sb.WriteString("Provide a detailed annotation in English JSON format with these exact fields:\n")
	}
	sb.WriteString("- translation: Direct English translation of the selected Japanese text\n")
	sb.WriteString("- contextual_explanation: Explanation of what it means in this page/document context\n")
	sb.WriteString("- usage_example: Example sentence showing how to use this in a professional/work context\n")
	sb.WriteString("- when_to_use: When and in what situation this phrase is used\n")
	sb.WriteString("- word_breakdown: Single JSON string explaining each word/component in the selected text; do not return an object or array\n")
	sb.WriteString("- alternative_meanings: Alternative meanings in different fields or contexts\n")
	sb.WriteString("- pronunciation: object with kana reading and optional romaji\n\n")
	sb.WriteString("Return only valid JSON, no markdown formatting.")

	return sb.String()
}

func buildSpeechPrompt(highlightedText string, contextText string) string {
	context := strings.TrimSpace(contextText)
	if len(context) > 400 {
		context = context[:400]
	}

	return fmt.Sprintf(
		`You are a Japanese text-to-speech assistant.
Speak only the "Selected text" exactly as written.
Use "Context" only to infer subtle, natural tone and pacing.
Do not read context out loud.
Avoid exaggerated acting; keep delivery clear and realistic.

Selected text:
%s

Context:
%s`,
		highlightedText,
		context,
	)
}

func wrapPCMAsWAV(pcm []byte, sampleRate, channels, bitsPerSample int) []byte {
	dataLen := len(pcm)
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	chunkSize := 36 + dataLen

	buf := bytes.NewBuffer(make([]byte, 0, 44+dataLen))
	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, uint32(chunkSize))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, uint16(channels))
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(buf, binary.LittleEndian, uint32(byteRate))
	_ = binary.Write(buf, binary.LittleEndian, uint16(blockAlign))
	_ = binary.Write(buf, binary.LittleEndian, uint16(bitsPerSample))
	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, uint32(dataLen))
	buf.Write(pcm)

	return buf.Bytes()
}

func normalizeAnnotationResponse(annotation *AnnotationResponse) *AnnotationResponse {
	if annotation.Meaning == "" {
		if annotation.ContextualExplanation != "" {
			annotation.Meaning = annotation.ContextualExplanation
		} else {
			annotation.Meaning = annotation.Translation
		}
	}
	if annotation.ContextualExplanation == "" {
		annotation.ContextualExplanation = annotation.Meaning
	}
	if annotation.Translation == "" {
		annotation.Translation = annotation.Meaning
	}
	if annotation.AlternativeMeanings == "" {
		annotation.AlternativeMeanings = annotation.AlternativeMeaning
	}
	return annotation
}

func decodeAnnotationJSON(text string) (*AnnotationResponse, error) {
	candidates := []string{text}
	normalized := normalizeJSONCandidate(text)
	if normalized != text {
		candidates = append(candidates, normalized)
	}

	var lastErr error
	for _, candidate := range candidates {
		normalizedJSON, err := normalizeAnnotationJSONStringFields([]byte(candidate))
		if err != nil {
			lastErr = err
			continue
		}
		var annotation AnnotationResponse
		if err := json.Unmarshal(normalizedJSON, &annotation); err != nil {
			lastErr = err
			continue
		}
		return normalizeAnnotationResponse(&annotation), nil
	}
	return nil, lastErr
}

func normalizeAnnotationJSONStringFields(payload []byte) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return nil, err
	}

	for _, key := range []string{"word_breakdown"} {
		raw, ok := fields[key]
		if !ok || len(raw) == 0 || raw[0] == '"' {
			continue
		}
		encoded, err := json.Marshal(string(raw))
		if err != nil {
			return nil, err
		}
		fields[key] = encoded
	}

	return json.Marshal(fields)
}

func normalizeJSONCandidate(s string) string {
	trimmed := strings.TrimSpace(s)
	if strings.HasPrefix(trimmed, "```") {
		if idx := strings.Index(trimmed, "\n"); idx != -1 {
			trimmed = strings.TrimSpace(trimmed[idx+1:])
		}
		trimmed = strings.TrimSuffix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
	}

	start := strings.IndexByte(trimmed, '{')
	end := strings.LastIndexByte(trimmed, '}')
	if start != -1 && end != -1 && end > start {
		return strings.TrimSpace(trimmed[start : end+1])
	}
	return strings.TrimSpace(s)
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

package ai

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gemini-hackathon/app/internal/knowledge"
	"google.golang.org/genai"
)

func TestClientOCRUsesOpenRouterVisionRequest(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %s, want /chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer openrouter-key" {
			t.Fatalf("Authorization = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"raw_text\":\"小川未明\",\"language\":\"ja\"}"}}]}`))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		AIProvider:         ProviderMiniMax,
		OpenRouterAPIKey:   "openrouter-key",
		OpenRouterBaseURL:  server.URL,
		OpenRouterOCRModel: "baidu/qianfan-ocr-fast:free",
		MiniMaxAPIKey:      "minimax-key",
		HTTPClient:         server.Client(),
	})

	resp, err := client.OCR(context.Background(), []byte("image"), "image/jpeg")
	if err != nil {
		t.Fatalf("OCR returned error: %v", err)
	}
	if resp.RawText != "小川未明" {
		t.Fatalf("RawText = %q", resp.RawText)
	}
	if captured["model"] != "baidu/qianfan-ocr-fast:free" {
		t.Fatalf("model = %v", captured["model"])
	}
	body, err := json.Marshal(captured)
	if err != nil {
		t.Fatalf("marshal captured request: %v", err)
	}
	if !strings.Contains(string(body), "data:image/jpeg;base64,") {
		t.Fatalf("request did not include base64 image data: %s", string(body))
	}
}

func TestClientAnnotateWithKnowledgeUsesMiniMaxAnthropicRequest(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			t.Fatalf("path = %s, want /messages", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer minimax-key" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got == "" {
			t.Fatal("expected anthropic-version header")
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"{\"translation\":\"Ogawa Mimei\",\"contextual_explanation\":\"Author name\",\"usage_example\":\"小川未明を読みます。\",\"when_to_use\":\"When referring to the author\",\"word_breakdown\":\"小川: surname; 未明: given name\",\"alternative_meanings\":\"Proper noun\",\"pronunciation\":{\"kana\":\"おがわみめい\",\"romaji\":\"Ogawa Mimei\"}}"}]}`))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		AIProvider:              ProviderMiniMax,
		OpenRouterAPIKey:        "openrouter-key",
		MiniMaxAPIKey:           "minimax-key",
		MiniMaxAnthropicBaseURL: server.URL,
		MiniMaxTextModel:        "MiniMax-M2.7",
		HTTPClient:              server.Client(),
	})

	resp, err := client.AnnotateWithKnowledge(context.Background(), "context", "小川未明", []knowledge.Entry{
		{Kosakata: "小川未明", Kana: "おがわみめい", Arti: "Ogawa Mimei"},
	})
	if err != nil {
		t.Fatalf("AnnotateWithKnowledge returned error: %v", err)
	}
	if resp.Meaning == "" {
		t.Fatal("expected normalized meaning")
	}
	if captured["model"] != "MiniMax-M2.7" {
		t.Fatalf("model = %v", captured["model"])
	}
}

func TestClientAnnotateWithKnowledgeAcceptsObjectWordBreakdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"{\"translation\":\"sleepy town\",\"contextual_explanation\":\"A title phrase\",\"usage_example\":\"眠い町を読みます。\",\"when_to_use\":\"When discussing the story title\",\"word_breakdown\":{\"眠い\":\"sleepy\",\"町\":\"town\"},\"alternative_meanings\":\"None\",\"pronunciation\":{\"kana\":\"ねむいまち\"}}"}]}`))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		AIProvider:              ProviderMiniMax,
		MiniMaxAPIKey:           "minimax-key",
		MiniMaxAnthropicBaseURL: server.URL,
		HTTPClient:              server.Client(),
	})

	resp, err := client.AnnotateWithKnowledge(context.Background(), "context", "眠い町", nil)
	if err != nil {
		t.Fatalf("AnnotateWithKnowledge returned error: %v", err)
	}
	if !strings.Contains(resp.WordBreakdown, "眠い") || !strings.Contains(resp.WordBreakdown, "sleepy") {
		t.Fatalf("WordBreakdown = %q, want normalized object contents", resp.WordBreakdown)
	}
}

func TestClientGeminiProviderRequiresGeminiKey(t *testing.T) {
	client := NewClient(ClientConfig{AIProvider: ProviderGemini})

	_, err := client.OCR(context.Background(), []byte("image"), "image/jpeg")
	if err == nil {
		t.Fatal("expected missing Gemini key to fail")
	}

	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("error = %T, want ProviderError", err)
	}
	if providerErr.Provider != ProviderGemini {
		t.Fatalf("Provider = %q, want %q", providerErr.Provider, ProviderGemini)
	}
}

func TestClientSynthesizeSpeechUsesMiniMaxTTS(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/t2a_v2" {
			t.Fatalf("path = %s, want /t2a_v2", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer minimax-key" {
			t.Fatalf("Authorization = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"audio":"0102ff","status":2},"extra_info":{"audio_format":"mp3"},"base_resp":{"status_code":0,"status_msg":"success"}}`))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		AIProvider:        ProviderMiniMax,
		OpenRouterAPIKey:  "openrouter-key",
		MiniMaxAPIKey:     "minimax-key",
		MiniMaxTTSBaseURL: server.URL,
		MiniMaxTTSModel:   "speech-2.8-hd",
		MiniMaxTTSVoiceID: "Japanese_Whisper_Belle",
		HTTPClient:        server.Client(),
	})

	resp, err := client.SynthesizeSpeech(context.Background(), "おはよう", "朝の挨拶")
	if err != nil {
		t.Fatalf("SynthesizeSpeech returned error: %v", err)
	}
	if string(resp.Audio) != string([]byte{0x01, 0x02, 0xff}) {
		t.Fatalf("audio bytes = %v", resp.Audio)
	}
	if resp.MIMEType != "audio/mpeg" {
		t.Fatalf("MIMEType = %q", resp.MIMEType)
	}
	if captured["model"] != "speech-2.8-hd" {
		t.Fatalf("model = %v", captured["model"])
	}
	body, err := json.Marshal(captured)
	if err != nil {
		t.Fatalf("marshal captured request: %v", err)
	}
	if !strings.Contains(string(body), "Japanese_Whisper_Belle") {
		t.Fatalf("request did not include voice id: %s", string(body))
	}
}

func TestClassifyMiniMaxStatusMapsTPMRateLimit(t *testing.T) {
	if got := classifyMiniMaxStatus(1039, "triggered tpm rate limit"); got != ErrorKindRateLimit {
		t.Fatalf("classifyMiniMaxStatus(1039) = %s, want %s", got, ErrorKindRateLimit)
	}
}

func TestClassifyMiniMaxStatusMapsUnsupportedModelToConfig(t *testing.T) {
	if got := classifyMiniMaxStatus(0, "your current token plan not support model, speech-2.8-turbo"); got != ErrorKindConfig {
		t.Fatalf("classifyMiniMaxStatus() = %s, want %s", got, ErrorKindConfig)
	}
}

func TestGeminiSpeechRequestSendsOnlySelectionWithStyleMetadata(t *testing.T) {
	contents, cfg := geminiSpeechRequest("今日は雨です")

	if len(contents) != 1 || len(contents[0].Parts) != 1 {
		t.Fatalf("contents = %+v, want one content with one part", contents)
	}
	part := contents[0].Parts[0]
	if part.Text != "今日は雨です" {
		t.Fatalf("part text = %q, want only the selected text", part.Text)
	}
	if part.SpeechMetadata == nil || part.SpeechMetadata.Style == "" {
		t.Fatal("delivery style must be sent as speech metadata, not spoken text")
	}
	if geminiSpeechModel != "gemini-3.8-flash-tts" {
		t.Fatalf("geminiSpeechModel = %q, want gemini-3.8-flash-tts", geminiSpeechModel)
	}
	if cfg.SpeechConfig == nil || cfg.SpeechConfig.LanguageCode != "ja-JP" {
		t.Fatalf("speech config = %+v, want ja-JP", cfg.SpeechConfig)
	}
	if voice := cfg.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig.VoiceName; voice != "Kore" {
		t.Fatalf("voice = %q, want Kore", voice)
	}
	if len(cfg.ResponseModalities) != 1 || cfg.ResponseModalities[0] != "AUDIO" {
		t.Fatalf("ResponseModalities = %v, want [AUDIO]", cfg.ResponseModalities)
	}
}

func TestGeminiSpeechAudio(t *testing.T) {
	pcm := []byte{1, 2, 3, 4}
	wav := wrapPCMAsWAV(pcm, 24000, 1, 16)

	tests := []struct {
		name     string
		data     []byte
		mimeType string
		wantMIME string
		wantRate uint32
		wantSame bool
	}{
		{name: "wav default passes through", data: wav, mimeType: "audio/wav", wantMIME: "audio/wav", wantSame: true},
		{name: "riff detected without mime", data: wav, mimeType: "", wantMIME: "audio/wav", wantSame: true},
		{name: "legacy pcm mime is wrapped", data: pcm, mimeType: "audio/L16;codec=pcm;rate=24000", wantMIME: "audio/wav", wantRate: 24000},
		{name: "bare l16 is wrapped at its rate", data: pcm, mimeType: "audio/l16; rate=16000", wantMIME: "audio/wav", wantRate: 16000},
		{name: "empty mime is wrapped", data: pcm, mimeType: "", wantMIME: "audio/wav", wantRate: 24000},
		{name: "other formats pass through", data: pcm, mimeType: "audio/mulaw", wantMIME: "audio/mulaw", wantSame: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			audio, mimeType := geminiSpeechAudio(tt.data, tt.mimeType)
			if mimeType != tt.wantMIME {
				t.Fatalf("mime = %q, want %q", mimeType, tt.wantMIME)
			}
			if tt.wantSame {
				if string(audio) != string(tt.data) {
					t.Fatal("audio should pass through unchanged")
				}
				return
			}
			if len(audio) != 44+len(tt.data) || string(audio[0:4]) != "RIFF" {
				t.Fatalf("audio was not wrapped as WAV: %v", audio)
			}
			if rate := binary.LittleEndian.Uint32(audio[24:28]); rate != tt.wantRate {
				t.Fatalf("sample rate = %d, want %d", rate, tt.wantRate)
			}
		})
	}
}

func TestGeminiSynthesizeSpeechSendsVerbatimTranscriptAndKeepsWAV(t *testing.T) {
	wav := wrapPCMAsWAV([]byte{1, 2, 3, 4}, 24000, 1, 16)
	var path string
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []any{map[string]any{
				"content": map[string]any{"role": "model", "parts": []any{map[string]any{
					"inlineData": map[string]any{"mimeType": "audio/wav", "data": base64.StdEncoding.EncodeToString(wav)},
				}}},
			}},
		})
	}))
	defer server.Close()

	genaiClient, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:      "test-key",
		Backend:     genai.BackendGeminiAPI,
		HTTPOptions: genai.HTTPOptions{BaseURL: server.URL},
	})
	if err != nil {
		t.Fatal(err)
	}
	client := &geminiClient{apiKey: "test-key", genaiClient: genaiClient}

	resp, err := client.SynthesizeSpeech(context.Background(), "図書館", "駅前の小さな図書館は、朝九時に開きます。")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "/models/gemini-3.8-flash-tts:generateContent") {
		t.Fatalf("path = %s, want gemini-3.8-flash-tts generateContent", path)
	}
	body, err := json.Marshal(captured)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "駅前") || strings.Contains(string(body), "Selected text") {
		t.Fatalf("request must not contain context or prompt directions: %s", body)
	}
	contents, ok := captured["contents"].([]any)
	if !ok || len(contents) != 1 {
		t.Fatalf("contents = %v", captured["contents"])
	}
	content, _ := contents[0].(map[string]any)
	parts, _ := content["parts"].([]any)
	if len(parts) != 1 {
		t.Fatalf("parts = %v", content["parts"])
	}
	part, _ := parts[0].(map[string]any)
	if part["text"] != "図書館" {
		t.Fatalf("text = %v, want only the selection", part["text"])
	}
	metadata, _ := part["speechMetadata"].(map[string]any)
	if style, _ := metadata["style"].(string); style == "" {
		t.Fatalf("speechMetadata.style missing from request: %s", body)
	}
	if resp.MIMEType != "audio/wav" || string(resp.Audio) != string(wav) {
		t.Fatalf("response = %q (%d bytes), want unchanged WAV", resp.MIMEType, len(resp.Audio))
	}
}

// Providers can echo request content or credential fragments in error
// bodies. ProviderError text is logged, so it must carry only the
// classification, never provider-supplied text.
const (
	errorBodySecretSentinel  = "sk-or-v1-SENTINEL-CREDENTIAL"
	errorBodyContentSentinel = "SENTINEL-PRIVATE-PAGE-TEXT"
)

func assertNoProviderText(t *testing.T, err error) *ProviderError {
	t.Helper()
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("error = %T (%v), want ProviderError", err, err)
	}
	for _, sentinel := range []string{errorBodySecretSentinel, errorBodyContentSentinel} {
		if strings.Contains(err.Error(), sentinel) || strings.Contains(providerErr.Message, sentinel) {
			t.Fatalf("provider text leaked into error %q", err.Error())
		}
	}
	return providerErr
}

func TestLegacyHTTPErrorsDoNotKeepProviderBodies(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantKind ErrorKind
	}{
		{"auth", http.StatusUnauthorized, `{"error":{"message":"Invalid key ` + errorBodySecretSentinel + `"}}`, ErrorKindAuth},
		{"quota still classified", http.StatusPaymentRequired, `{"error":"quota exceeded for ` + errorBodyContentSentinel + `"}`, ErrorKindQuota},
		{"echoed request", http.StatusBadRequest, `{"error":{"message":"bad input: ` + errorBodyContentSentinel + `"}}`, ErrorKindUpstream},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient(ClientConfig{
				AIProvider:        ProviderMiniMax,
				OpenRouterAPIKey:  "openrouter-key",
				OpenRouterBaseURL: server.URL,
				HTTPClient:        server.Client(),
			})
			_, err := client.OCR(context.Background(), []byte("image"), "image/jpeg")

			providerErr := assertNoProviderText(t, err)
			if providerErr.Kind != tt.wantKind || providerErr.StatusCode != tt.status {
				t.Fatalf("kind/status = %s/%d, want %s/%d", providerErr.Kind, providerErr.StatusCode, tt.wantKind, tt.status)
			}
		})
	}
}

func TestMiniMaxStatusErrorsDoNotKeepProviderMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"base_resp":{"status_code":1004,"status_msg":"login fail ` + errorBodySecretSentinel + `"}}`))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		AIProvider:        ProviderMiniMax,
		MiniMaxAPIKey:     "minimax-key",
		MiniMaxTTSBaseURL: server.URL,
		HTTPClient:        server.Client(),
	})
	_, err := client.SynthesizeSpeech(context.Background(), "おはよう", "")

	if providerErr := assertNoProviderText(t, err); providerErr.Kind != ErrorKindAuth {
		t.Fatalf("Kind = %s, want auth", providerErr.Kind)
	}
}

func TestGeminiErrorsDoNotKeepProviderMessage(t *testing.T) {
	sdkErr := genai.APIError{Code: http.StatusForbidden, Status: "403 Forbidden", Message: "permission denied for " + errorBodySecretSentinel + " " + errorBodyContentSentinel}

	err := wrapGeminiError("failed to generate annotation", sdkErr)

	providerErr := assertNoProviderText(t, err)
	if providerErr.Kind != ErrorKindAuth || providerErr.StatusCode != http.StatusForbidden {
		t.Fatalf("kind/status = %s/%d, want auth/403", providerErr.Kind, providerErr.StatusCode)
	}
	var cause genai.APIError
	if !errors.As(err, &cause) || cause.Code != http.StatusForbidden {
		t.Fatal("SDK error should stay available as the unlogged cause")
	}
}

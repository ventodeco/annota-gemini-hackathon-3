package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gemini-hackathon/app/internal/knowledge"
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

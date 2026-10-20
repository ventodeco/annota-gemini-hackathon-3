package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// Editing a prompt changes its hash; bump the matching PromptVersion constant
// and update the pinned hash together so saved provenance stays truthful.
func TestPromptVersionsArePinned(t *testing.T) {
	pinned := []struct {
		version, prompt, hash string
	}{
		{PromptVersionOCROpenRouterJSON, openRouterOCRPrompt, "a29226b590b759e2825fc9503f7b395cbc58f73f3e51d250b5f24b36ca48b836"},
		{PromptVersionOCRGeminiJSON, geminiOCRPrompt, "bcca6b89411dd5b7fd6661be1034a289a68badeafc1b833e427d70b32522bc62"},
		{PromptVersionOCRRoutedJSON, routedOCRJSONPrompt, "4be6adb594ef2294e9cb8b8a17f69ec65311a8d02524ec03cd9bbf59acae8a9f"},
		{PromptVersionOCRRoutedMarkdown, routedOCRMarkdownPrompt, "af61fc50e7722e5fcc14205e32ccbe27bf9a7a37a506d585262a7a3eba105cdf"},
		{PromptVersionAnnotation, buildEnhancedPrompt("本文", "語", nil), "3c926e58976b0bef6c746b511f23e991f1adb7496494b0670394cda5d73c80ff"},
	}
	for _, p := range pinned {
		sum := sha256.Sum256([]byte(p.prompt))
		if got := hex.EncodeToString(sum[:]); got != p.hash {
			t.Errorf("%s prompt changed (hash %s); bump its version and update the pinned hash", p.version, got)
		}
	}
}

func TestProducerReportsConfiguredRoutes(t *testing.T) {
	legacy := NewClient(ClientConfig{OpenRouterOCRModel: "vendor/ocr-model", MiniMaxTextModel: "text-model"})
	gemini := NewClient(ClientConfig{AIProvider: ProviderGemini})
	routing := routingFixture(t)
	routed, err := NewRoutedClient(routing, RoutedClientOptions{})
	if err != nil {
		t.Fatal(err)
	}
	markdownRouting := routingFixture(t)
	model := markdownRouting.Profiles["active"].Models["test-vision"]
	model.OCROutput = "markdown"
	markdownRouting.Profiles["active"].Models["test-vision"] = model
	markdownRouted, err := NewRoutedClient(markdownRouting, RoutedClientOptions{})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		client Client
		task   string
		want   Producer
	}{
		{"legacy ocr", legacy, "ocr", Producer{Provider: ProviderMiniMax, Gateway: "openrouter", Model: "vendor/ocr-model", PromptVersion: PromptVersionOCROpenRouterJSON}},
		{"legacy annotation", legacy, "annotation", Producer{Provider: ProviderMiniMax, Gateway: "minimax", Model: "text-model", PromptVersion: PromptVersionAnnotation}},
		{"gemini ocr", gemini, "ocr", Producer{Provider: ProviderGemini, Gateway: "google", Model: "gemini-2.5-flash", PromptVersion: PromptVersionOCRGeminiJSON}},
		{"gemini annotation", gemini, "annotation", Producer{Provider: ProviderGemini, Gateway: "google", Model: "gemini-2.5-flash", PromptVersion: PromptVersionAnnotation}},
		{"routed ocr", routed, "ocr", Producer{Provider: ProviderRouted, Gateway: "openrouter", Region: "global", Model: "test-vision", PromptVersion: PromptVersionOCRRoutedJSON}},
		{"routed markdown ocr", markdownRouted, "ocr", Producer{Provider: ProviderRouted, Gateway: "openrouter", Region: "global", Model: "test-vision", PromptVersion: PromptVersionOCRRoutedMarkdown}},
		{"routed annotation", routed, "annotation", Producer{Provider: ProviderRouted, Gateway: "openrouter", Region: "global", Model: "test-vision", PromptVersion: PromptVersionAnnotation}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ProducerFor(tt.client, tt.task)
			if !ok || got != tt.want {
				t.Fatalf("ProducerFor = %+v, %v; want %+v", got, ok, tt.want)
			}
		})
	}
	if _, ok := ProducerFor(routed, "speech"); ok {
		t.Fatal("speech output is not persisted and has no producer record")
	}
}

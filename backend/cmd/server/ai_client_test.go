package main

import (
	"context"
	"testing"

	"github.com/gemini-hackathon/app/internal/ai"
	"github.com/gemini-hackathon/app/internal/config"
)

func TestBuildAIClientRejectsInvalidModeBeforeConnections(t *testing.T) {
	for _, cfg := range []*config.Config{{AIProvider: "routed"}, {AIProvider: "unknown"}} {
		if _, err := buildAIClient(cfg); err == nil {
			t.Fatal("invalid mode accepted")
		}
	}
}

func TestBuildAIClientRetainsLegacyModes(t *testing.T) {
	for _, mode := range []string{"gemini", "minimax", ""} {
		client, err := buildAIClient(&config.Config{AIProvider: mode})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.OCR(context.Background(), []byte("page"), "image/png"); err == nil {
			t.Fatal("missing provider key ignored")
		}
	}
	cfg, err := ai.LoadRoutingConfig("../../internal/ai/testdata/routing.json", func(string) string { return "fixture-key" })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildAIClient(&config.Config{AIProvider: "routed", AIRouting: &cfg}); err != nil {
		t.Fatal(err)
	}
}

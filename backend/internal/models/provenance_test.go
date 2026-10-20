package models

import (
	"strings"
	"testing"
)

func TestAIProvenanceValid(t *testing.T) {
	valid := AIProvenance{Version: 1, Provider: "routed", Gateway: "openrouter", Region: "global", Model: "google/gemini-3.5-flash-lite", ReportedModel: "google/gemini-3.5-flash-lite-20260721", PromptVersion: "annotation/1"}
	mutate := func(change func(*AIProvenance)) *AIProvenance {
		p := valid
		change(&p)
		return &p
	}
	tests := []struct {
		name string
		p    *AIProvenance
		want bool
	}{
		{"valid routed", &valid, true},
		{"legacy without region", mutate(func(p *AIProvenance) { p.Provider, p.Region, p.ReportedModel = "minimax", "", "" }), true},
		{"nil", nil, false},
		{"unknown version", mutate(func(p *AIProvenance) { p.Version = 2 }), false},
		{"unknown provider", mutate(func(p *AIProvenance) { p.Provider = "other" }), false},
		{"missing model", mutate(func(p *AIProvenance) { p.Model = "" }), false},
		{"missing prompt version", mutate(func(p *AIProvenance) { p.PromptVersion = "" }), false},
		{"text in model", mutate(func(p *AIProvenance) { p.Model = "駅前の図書館" }), false},
		{"spaces in gateway", mutate(func(p *AIProvenance) { p.Gateway = "open router" }), false},
		{"oversized reported model", mutate(func(p *AIProvenance) { p.ReportedModel = strings.Repeat("a", 129) }), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Valid(); got != tt.want {
				t.Fatalf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

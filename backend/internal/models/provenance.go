package models

import "regexp"

// AIProvenanceVersion is the schema version of AIProvenance.
const AIProvenanceVersion = 1

var provenanceToken = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@-]{0,127}$`)

// AIProvenance identifies the configured producer of a saved AI output so later
// output differences can be explained. It holds identifiers only: never keys,
// prompts, request IDs, source text or usage. ReportedModel is set only when the
// provider reported it; historical rows have no provenance (unknown).
type AIProvenance struct {
	Version       int    `json:"version"`
	Provider      string `json:"provider"`
	Gateway       string `json:"gateway,omitempty"`
	Region        string `json:"region,omitempty"`
	Model         string `json:"model"`
	ReportedModel string `json:"reportedModel,omitempty"`
	PromptVersion string `json:"promptVersion"`
}

// Valid reports whether p is a well-formed version-1 provenance record with
// identifier-shaped values from a known provider mode.
func (p *AIProvenance) Valid() bool {
	if p == nil || p.Version != AIProvenanceVersion {
		return false
	}
	switch p.Provider {
	case "minimax", "gemini", "routed":
	default:
		return false
	}
	for _, required := range []string{p.Model, p.PromptVersion} {
		if !provenanceToken.MatchString(required) {
			return false
		}
	}
	for _, optional := range []string{p.Gateway, p.Region, p.ReportedModel} {
		if optional != "" && !provenanceToken.MatchString(optional) {
			return false
		}
	}
	return true
}

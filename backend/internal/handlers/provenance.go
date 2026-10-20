package handlers

import (
	"github.com/gemini-hackathon/app/internal/ai"
	"github.com/gemini-hackathon/app/internal/models"
)

// provenanceFor returns the configured producer of task output, or nil when the
// client does not report one so the output is stored as unknown.
func provenanceFor(client ai.Client, task string) *models.AIProvenance {
	if client == nil {
		return nil
	}
	producer, ok := ai.ProducerFor(client, task)
	if !ok {
		return nil
	}
	provenance := &models.AIProvenance{
		Version:       models.AIProvenanceVersion,
		Provider:      producer.Provider,
		Gateway:       producer.Gateway,
		Region:        producer.Region,
		Model:         producer.Model,
		PromptVersion: producer.PromptVersion,
	}
	if !provenance.Valid() {
		return nil
	}
	return provenance
}

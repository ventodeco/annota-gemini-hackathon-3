package services

import (
	"github.com/gemini-hackathon/app/internal/ai"
	"github.com/gemini-hackathon/app/internal/models"
)

// ProvenanceFor returns the configured producer of task output, or nil when the
// client does not report one so the output is stored as unknown. calls are the
// request's recorded gateway attempts (usage.Calls); the provider-reported model
// is added only from the final successful attempt for task, when approved.
func ProvenanceFor(client ai.Client, task string, calls []ai.CallMetadata) *models.AIProvenance {
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
		ReportedModel: reportedModel(client, task, calls),
	}
	if !provenance.Valid() {
		return nil
	}
	return provenance
}

// reportedModel returns the model the gateway reported for the last attempt of
// task, if that attempt succeeded and the model is approved for the route.
// Anything else stays unknown (empty) rather than assuming the configured model.
func reportedModel(client ai.Client, task string, calls []ai.CallMetadata) string {
	for i := len(calls) - 1; i >= 0; i-- {
		if calls[i].Task != task {
			continue
		}
		if calls[i].ErrorKind == "" && ai.ReportedModelAllowed(client, task, calls[i].ReportedModel) {
			return calls[i].ReportedModel
		}
		return ""
	}
	return ""
}

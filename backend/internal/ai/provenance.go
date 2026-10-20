package ai

import "slices"

// Prompt versions identify the prompt text behind a saved output. Bump the
// version whenever the corresponding prompt changes; TestPromptVersionsArePinned
// fails until the version and its pinned hash are updated together.
const (
	PromptVersionAnnotation        = "annotation/1"
	PromptVersionOCROpenRouterJSON = "ocr-openrouter-json/1"
	PromptVersionOCRGeminiJSON     = "ocr-gemini-json/1"
	PromptVersionOCRRoutedJSON     = "ocr-routed-json/1"
	PromptVersionOCRRoutedMarkdown = "ocr-routed-markdown/1"
	openRouterOCRPrompt            = "Extract all readable text from this image. Return only JSON with keys raw_text and language. Use language code ja for Japanese."
	geminiOCRPrompt                = "Extract all Japanese text from this image. Return ONLY a JSON object with keys raw_text and language. Use language code ja for Japanese. Preserve line breaks and formatting."
	routedOCRJSONPrompt            = "Extract all readable Japanese text. Return only JSON with raw_text and language. Preserve line breaks."
	routedOCRMarkdownPrompt        = "Extract all readable Japanese text as Markdown. Preserve reading order and line breaks. Return only the transcript."
)

// Producer describes the configured producer of a task's output: the provider
// mode, gateway, region, model ID and prompt version. It is configuration, not
// what a provider reported.
type Producer struct {
	Provider      string
	Gateway       string
	Region        string
	Model         string
	PromptVersion string
}

// ProducerReporter is implemented by clients that can describe their producer
// for "ocr" and "annotation" tasks.
type ProducerReporter interface {
	Producer(task string) (Producer, bool)
}

// ProducerFor returns the client's producer for task, or false when the client
// does not report one (for example test doubles) or the task is unknown.
func ProducerFor(c Client, task string) (Producer, bool) {
	reporter, ok := c.(ProducerReporter)
	if !ok {
		return Producer{}, false
	}
	return reporter.Producer(task)
}

func (c *client) Producer(task string) (Producer, bool) {
	switch task {
	case "ocr":
		return Producer{Provider: ProviderMiniMax, Gateway: "openrouter", Model: c.cfg.OpenRouterOCRModel, PromptVersion: PromptVersionOCROpenRouterJSON}, true
	case "annotation":
		return Producer{Provider: ProviderMiniMax, Gateway: "minimax", Model: c.cfg.MiniMaxTextModel, PromptVersion: PromptVersionAnnotation}, true
	}
	return Producer{}, false
}

func (c *geminiClient) Producer(task string) (Producer, bool) {
	switch task {
	case "ocr":
		return Producer{Provider: ProviderGemini, Gateway: "google", Model: c.modelName, PromptVersion: PromptVersionOCRGeminiJSON}, true
	case "annotation":
		return Producer{Provider: ProviderGemini, Gateway: "google", Model: c.modelName, PromptVersion: PromptVersionAnnotation}, true
	}
	return Producer{}, false
}

func (c *routedClient) Producer(task string) (Producer, bool) {
	switch task {
	case "ocr":
		version := PromptVersionOCRRoutedJSON
		if c.ocr.model.OCROutput == "markdown" {
			version = PromptVersionOCRRoutedMarkdown
		}
		return c.ocr.producer(version), true
	case "annotation":
		return c.annotation.producer(PromptVersionAnnotation), true
	}
	return Producer{}, false
}

func (g *taskGateway) producer(promptVersion string) Producer {
	return Producer{Provider: ProviderRouted, Gateway: g.profile.Kind, Region: g.profile.Region, Model: g.route.Model, PromptVersion: promptVersion}
}

// ReportedModelAllowed reports whether reported is a model ID the client's task
// route accepts from the gateway: its selected model or a documented alias.
// Only routed clients receive reported models, so others always return false.
func ReportedModelAllowed(c Client, task, reported string) bool {
	routed, ok := c.(*routedClient)
	if !ok || reported == "" {
		return false
	}
	var g *taskGateway
	switch task {
	case "ocr":
		g = routed.ocr
	case "annotation":
		g = routed.annotation
	default:
		return false
	}
	return reported == g.route.Model || slices.Contains(g.model.ReportedModelAliases, reported)
}

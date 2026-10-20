package ai

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
)

const ProviderRouted = "routed"

// RoutingConfig is immutable after startup. Keys are resolved only for active profiles.
type RoutingConfig struct {
	Version  int                       `json:"version"`
	Profiles map[string]GatewayProfile `json:"profiles"`
	Tasks    TaskRoutes                `json:"tasks"`
}

type GatewayProfile struct {
	Kind           string                       `json:"kind"`
	Region         string                       `json:"region"`
	BaseURL        string                       `json:"base_url"`
	APIKeyEnv      string                       `json:"api_key_env"`
	APIKey         string                       `json:"-"`
	TimeoutSeconds int                          `json:"timeout_seconds"`
	MaxAttempts    int                          `json:"max_attempts"`
	Models         map[string]ModelCapabilities `json:"models"`
}

type ModelCapabilities struct {
	ImageInput           bool     `json:"image_input,omitempty"`
	TextInput            bool     `json:"text_input,omitempty"`
	TextOutput           bool     `json:"text_output,omitempty"`
	AudioOutput          bool     `json:"audio_output,omitempty"`
	JSONMode             string   `json:"json_mode,omitempty"`
	OCROutput            string   `json:"ocr_output,omitempty"`
	MaxOutputTokens      int      `json:"max_output_tokens,omitempty"`
	JapaneseVoices       []string `json:"japanese_voices,omitempty"`
	AudioFormats         []string `json:"audio_formats,omitempty"`
	ReportedModelAliases []string `json:"reported_model_aliases,omitempty"`
}

type TaskRoutes struct {
	OCR        TaskRoute `json:"ocr"`
	Annotation TaskRoute `json:"annotation"`
	Speech     TaskRoute `json:"speech"`
}

type TaskRoute struct {
	Profile string `json:"profile"`
	Model   string `json:"model"`
	Voice   string `json:"voice,omitempty"`
	Format  string `json:"format,omitempty"`
}

func (t TaskRoutes) named() map[string]TaskRoute {
	return map[string]TaskRoute{"ocr": t.OCR, "annotation": t.Annotation, "speech": t.Speech}
}

// LoadRoutingConfig never includes file contents, URLs or credentials in errors.
func LoadRoutingConfig(path string, lookupEnv func(string) string) (RoutingConfig, error) {
	var cfg RoutingConfig
	f, err := os.Open(path)
	if err != nil {
		return cfg, fmt.Errorf("cannot open AI_ROUTING_CONFIG file")
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("AI routing file must match the version 1 JSON schema")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return cfg, fmt.Errorf("AI routing file must contain one JSON object")
	}
	// Check the real size too: a valid object followed by excessive whitespace is invalid.
	info, err := f.Stat()
	if err != nil || info.Size() > 1<<20 {
		return cfg, fmt.Errorf("AI routing file exceeds 1 MiB or cannot be read")
	}
	for id, p := range cfg.Profiles {
		if p.TimeoutSeconds == 0 {
			p.TimeoutSeconds = 60
		}
		if p.MaxAttempts == 0 {
			p.MaxAttempts = 1
		}
		cfg.Profiles[id] = p
	}
	// Validate declarations before looking up environment variables.
	if err := validateRouting(cfg, false); err != nil {
		return cfg, err
	}
	for _, route := range cfg.Tasks.named() {
		p := cfg.Profiles[route.Profile]
		if p.APIKey == "" && lookupEnv != nil {
			p.APIKey = strings.TrimSpace(lookupEnv(p.APIKeyEnv))
			cfg.Profiles[route.Profile] = p
		}
	}
	return cfg, ValidateRoutingConfig(cfg)
}

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func ValidateRoutingConfig(cfg RoutingConfig) error { return validateRouting(cfg, true) }

func validateRouting(cfg RoutingConfig, requireKeys bool) error {
	if cfg.Version != 1 || len(cfg.Profiles) == 0 {
		return fmt.Errorf("AI routing requires version 1 and gateway profiles")
	}
	for id, p := range cfg.Profiles {
		if strings.TrimSpace(id) == "" || (p.Kind != "openrouter" && p.Kind != "modelverse") {
			return fmt.Errorf("AI routing profile requires an ID and supported gateway kind")
		}
		if strings.TrimSpace(p.Region) == "" || !envName.MatchString(p.APIKeyEnv) || len(p.Models) == 0 {
			return fmt.Errorf("AI routing profile requires region, api_key_env and model declarations")
		}
		if !validGatewayURL(p.BaseURL) {
			return fmt.Errorf("AI routing base_url must be HTTPS ending in /v1 (HTTP allowed only on loopback), without credentials, query or fragment")
		}
		if p.TimeoutSeconds < 1 || p.TimeoutSeconds > 120 || p.MaxAttempts < 1 || p.MaxAttempts > 2 {
			return fmt.Errorf("AI routing timeout_seconds must be 1..120 and max_attempts 1..2")
		}
		for modelID, m := range p.Models {
			if strings.TrimSpace(modelID) == "" || (!m.TextOutput && !m.AudioOutput) {
				return fmt.Errorf("AI routing models require an ID and an output capability")
			}
			if m.TextOutput && (m.MaxOutputTokens <= 0 || !slices.Contains([]string{"prompt", "json_object", "json_schema"}, m.JSONMode)) {
				return fmt.Errorf("AI routing chat models require max_output_tokens and a supported json_mode")
			}
			if m.OCROutput != "" && m.OCROutput != "json" && m.OCROutput != "markdown" {
				return fmt.Errorf("AI routing ocr_output must be json or markdown")
			}
			for _, format := range m.AudioFormats {
				if format != "mp3" {
					return fmt.Errorf("AI routing speech supports mp3 output only")
				}
			}
		}
	}
	for task, route := range cfg.Tasks.named() {
		p, ok := cfg.Profiles[route.Profile]
		if !ok {
			return fmt.Errorf("AI %s route must reference a configured profile", task)
		}
		m, ok := p.Models[route.Model]
		if !ok {
			return fmt.Errorf("AI %s model must be declared in its selected profile", task)
		}
		if requireKeys && strings.TrimSpace(p.APIKey) == "" {
			return fmt.Errorf("AI %s route requires the selected profile's API key", task)
		}
		switch task {
		case "ocr":
			if !m.ImageInput || !m.TextInput || !m.TextOutput || m.OCROutput == "" {
				return fmt.Errorf("AI OCR requires image/text input, text output and ocr_output")
			}
		case "annotation":
			if !m.TextInput || !m.TextOutput {
				return fmt.Errorf("AI annotation requires text input/output")
			}
		case "speech":
			if !m.TextInput || !m.AudioOutput || strings.TrimSpace(route.Voice) == "" || !slices.Contains(m.JapaneseVoices, route.Voice) || route.Format != "mp3" || !slices.Contains(m.AudioFormats, route.Format) {
				return fmt.Errorf("AI speech requires an approved Japanese voice and mp3 format for its model")
			}
		}
		if task != "speech" && (route.Voice != "" || route.Format != "") {
			return fmt.Errorf("AI %s route cannot contain speech options", task)
		}
	}
	return nil
}

func validGatewayURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return false
	}
	if u.Scheme != "https" {
		ip := net.ParseIP(u.Hostname())
		if u.Scheme != "http" || (u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback())) {
			return false
		}
	}
	return strings.TrimRight(u.Path, "/") == "/v1" || strings.TrimRight(u.Path, "/") == "/api/v1"
}

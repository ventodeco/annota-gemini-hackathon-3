package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func routingFixture(t *testing.T) RoutingConfig {
	t.Helper()
	cfg, err := LoadRoutingConfig("testdata/routing.json", func(name string) string {
		if name != "TEST_ACTIVE_GATEWAY_KEY" {
			t.Fatalf("unexpected credential lookup %q", name)
		}
		return "fixture-key"
	})
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestRoutingActiveCredentialsAndSerialization(t *testing.T) {
	cfg := routingFixture(t)
	if cfg.Profiles["active"].APIKey != "fixture-key" || cfg.Profiles["unused"].APIKey != "" {
		t.Fatal("wrong active key resolution")
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "fixture-key") {
		t.Fatal("serialized credentials")
	}
	// IDs belong to the selected profile, not to a source-code model catalog.
	cfg.Tasks.OCR.Model = "custom-model"
	profile := cfg.Profiles["active"]
	profile.Models["custom-model"] = profile.Models["test-vision"]
	if err := ValidateRoutingConfig(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestRoutingRejectsInvalidSelections(t *testing.T) {
	cases := []struct {
		name   string
		change func(*RoutingConfig)
	}{
		{"version", func(c *RoutingConfig) { c.Version = 2 }},
		{"missing profile", func(c *RoutingConfig) { c.Tasks.OCR.Profile = "absent" }},
		{"model from another profile", func(c *RoutingConfig) { c.Tasks.OCR.Model = "absent" }},
		{"missing annotation", func(c *RoutingConfig) { c.Tasks.Annotation = TaskRoute{} }},
		{"active key", func(c *RoutingConfig) { p := c.Profiles["active"]; p.APIKey = ""; c.Profiles["active"] = p }},
		{"text only OCR", func(c *RoutingConfig) {
			p := c.Profiles["active"]
			m := p.Models["test-vision"]
			m.ImageInput = false
			p.Models["test-vision"] = m
		}},
		{"JSON mode", func(c *RoutingConfig) {
			p := c.Profiles["active"]
			m := p.Models["test-vision"]
			m.JSONMode = "imaginary"
			p.Models["test-vision"] = m
		}},
		{"voice", func(c *RoutingConfig) { c.Tasks.Speech.Voice = "other-ja" }},
		{"format", func(c *RoutingConfig) { c.Tasks.Speech.Format = "pcm" }},
		{"timeout", func(c *RoutingConfig) { p := c.Profiles["active"]; p.TimeoutSeconds = -1; c.Profiles["active"] = p }},
		{"attempt count", func(c *RoutingConfig) { p := c.Profiles["active"]; p.MaxAttempts = 3; c.Profiles["active"] = p }},
		{"scheme", func(c *RoutingConfig) {
			p := c.Profiles["active"]
			p.BaseURL = "http://remote.test/v1"
			c.Profiles["active"] = p
		}},
		{"userinfo", func(c *RoutingConfig) {
			p := c.Profiles["active"]
			p.BaseURL = "https://secret@remote.test/v1"
			c.Profiles["active"] = p
		}},
		{"path", func(c *RoutingConfig) {
			p := c.Profiles["active"]
			p.BaseURL = "https://remote.test/v1/chat/completions"
			c.Profiles["active"] = p
		}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := routingFixture(t)
			tt.change(&cfg)
			if ValidateRoutingConfig(cfg) == nil {
				t.Fatal("invalid selection accepted")
			}
		})
	}
}

func TestRoutingStrictFileParsing(t *testing.T) {
	original, err := os.ReadFile("testdata/routing.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{"", "null", "[]", string(original) + " {}", strings.Replace(string(original), `"version": 1`, `"unknown": "secret-echo", "version": 1`, 1)} {
		path := filepath.Join(t.TempDir(), "routes.json")
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := LoadRoutingConfig(path, func(string) string { return "fixture-key" })
		if err == nil {
			t.Fatal("invalid file accepted")
		}
		if strings.Contains(err.Error(), "secret-echo") {
			t.Fatal("unsafe parse error")
		}
	}
}

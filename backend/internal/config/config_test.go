package config

import (
	"reflect"
	"testing"
)

func TestConfig_GetAllowedOriginsList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty uses defaults",
			input:    "",
			expected: []string{"http://localhost:5173", "http://localhost:3000"},
		},
		{
			name:     "single origin",
			input:    "https://app.example.com",
			expected: []string{"https://app.example.com"},
		},
		{
			name:     "multiple origins",
			input:    "https://a.com,https://b.com",
			expected: []string{"https://a.com", "https://b.com"},
		},
		{
			name:     "origins with spaces",
			input:    "https://a.com, https://b.com , https://c.com",
			expected: []string{"https://a.com", "https://b.com", "https://c.com"},
		},
		{
			name:     "wildcard",
			input:    "*",
			expected: []string{"*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{AllowedOrigins: tt.input}
			got := cfg.GetAllowedOriginsList()
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("GetAllowedOriginsList() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConfig_IsOriginAllowed(t *testing.T) {
	tests := []struct {
		name           string
		allowedOrigins string
		origin         string
		want           bool
	}{
		{
			name:           "wildcard allows any origin",
			allowedOrigins: "*",
			origin:         "https://any-domain.com",
			want:           true,
		},
		{
			name:           "allowed origin in list",
			allowedOrigins: "http://localhost:5173,http://localhost:3000",
			origin:         "http://localhost:5173",
			want:           true,
		},
		{
			name:           "disallowed origin not in list",
			allowedOrigins: "http://localhost:5173",
			origin:         "http://localhost:3000",
			want:           false,
		},
		{
			name:           "empty origin not allowed",
			allowedOrigins: "http://localhost:5173",
			origin:         "",
			want:           false,
		},
		{
			name:           "origin with different scheme",
			allowedOrigins: "https://app.com",
			origin:         "http://app.com",
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{AllowedOrigins: tt.allowedOrigins}
			if got := cfg.IsOriginAllowed(tt.origin); got != tt.want {
				t.Errorf("IsOriginAllowed(%q) = %v, want %v", tt.origin, got, tt.want)
			}
		})
	}
}

package knowledge

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStripNotionLinks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single link",
			input:    "Perbankan (https://www.notion.so/abc123)",
			expected: "Perbankan",
		},
		{
			name:     "multiple links",
			input:    "Perbankan (https://notion.so/a), Bisnis (https://notion.so/b)",
			expected: "Perbankan, Bisnis",
		},
		{
			name:     "no link",
			input:    "Perbankan, Bisnis",
			expected: "Perbankan, Bisnis",
		},
		{
			name:     "http link",
			input:    "Test (http://example.com/path)",
			expected: "Test",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripNotionLinks(tt.input)
			if result != tt.expected {
				t.Errorf("StripNotionLinks(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseAndStripLinks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "comma separated with links",
			input:    "Perbankan (https://notion.so/a), Bisnis (https://notion.so/b), Marketing",
			expected: []string{"Perbankan", "Bisnis", "Marketing"},
		},
		{
			name:     "simple comma separated",
			input:    "Financial, Manufacturing, Retail",
			expected: []string{"Financial", "Manufacturing", "Retail"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "single value",
			input:    "Perbankan",
			expected: []string{"Perbankan"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseAndStripLinks(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("parseAndStripLinks(%q) = %v; want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNewService(t *testing.T) {
	// Create a temporary CSV file for testing
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test_knowledge.csv")

	csvContent := `Kosakata,Kana,Arti (EN / ID),Cara Baca,Deskripsi,Bidang Pekerjaan,Industri,Konteks
請求,せいきゅう,"claim, meminta",seikyuu,permintaan atau klaim,"Perbankan (https://notion.so/a), Bisnis","Financial, Manufacturing",
見積もり,みつもり,estimate,mitsumori,perkiraan harga,Bisnis,Manufacturing,
`
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatalf("Failed to create test CSV: %v", err)
	}

	svc, err := NewService(csvPath)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// Test that entries were loaded
	csvSvc := svc.(*csvService)
	if len(csvSvc.entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(csvSvc.entries))
	}

	// Check first entry
	entry := csvSvc.entries[0]
	if entry.Kosakata != "請求" {
		t.Errorf("Expected Kosakata '請求', got %q", entry.Kosakata)
	}
	if entry.Kana != "せいきゅう" {
		t.Errorf("Expected Kana 'せいきゅう', got %q", entry.Kana)
	}
	if !reflect.DeepEqual(entry.BidangPekerjaan, []string{"Perbankan", "Bisnis"}) {
		t.Errorf("Expected BidangPekerjaan [Perbankan, Bisnis], got %v", entry.BidangPekerjaan)
	}
}

func TestNewServiceFileNotFound(t *testing.T) {
	_, err := NewService("/nonexistent/path/to/file.csv")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestLookup(t *testing.T) {
	// Create a temporary CSV file for testing
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test_knowledge.csv")

	csvContent := `Kosakata,Kana,Arti (EN / ID),Cara Baca,Deskripsi,Bidang Pekerjaan,Industri,Konteks
請求,せいきゅう,"claim, meminta",seikyuu,permintaan atau klaim,Perbankan,Financial,
請求書,せいきゅうしょ,invoice,seikyuusho,dokumen tagihan,Bisnis,Financial,
見積もり,みつもり,estimate,mitsumori,perkiraan harga,Bisnis,Manufacturing,
`
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatalf("Failed to create test CSV: %v", err)
	}

	svc, err := NewService(csvPath)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	tests := []struct {
		name          string
		text          string
		expectedCount int
		expectedTerms []string
	}{
		{
			name:          "exact match",
			text:          "請求",
			expectedCount: 2, // 請求 and 請求書 (contains 請求)
			expectedTerms: []string{"請求", "請求書"},
		},
		{
			name:          "exact match single",
			text:          "見積もり",
			expectedCount: 1,
			expectedTerms: []string{"見積もり"},
		},
		{
			name:          "substring match - text contains term",
			text:          "請求書を送る",
			expectedCount: 2, // 請求 and 請求書 are contained in the text
			expectedTerms: []string{"請求", "請求書"},
		},
		{
			name:          "no match",
			text:          "こんにちは",
			expectedCount: 0,
			expectedTerms: nil,
		},
		{
			name:          "empty string",
			text:          "",
			expectedCount: 0,
			expectedTerms: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := svc.Lookup(tt.text)
			if len(results) != tt.expectedCount {
				t.Errorf("Lookup(%q) returned %d results; want %d", tt.text, len(results), tt.expectedCount)
			}

			if tt.expectedTerms != nil {
				for _, term := range tt.expectedTerms {
					found := false
					for _, r := range results {
						if r.Kosakata == term {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Lookup(%q) missing expected term %q", tt.text, term)
					}
				}
			}
		})
	}
}

func TestNewEmptyService(t *testing.T) {
	svc := NewEmptyService()
	results := svc.Lookup("請求")
	if len(results) != 0 {
		t.Errorf("Empty service should return no results, got %d", len(results))
	}
}

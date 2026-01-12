package gemini

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func getAPIKey(t *testing.T) string {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	if apiKey == "" {
		envLocalPath := filepath.Join("..", "..", ".env.local")
		if file, err := os.Open(envLocalPath); err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, "GEMINI_API_KEY=") {
					apiKey = strings.TrimPrefix(line, "GEMINI_API_KEY=")
					apiKey = strings.Trim(apiKey, `"'"`)
					break
				}
				if strings.HasPrefix(line, "GOOGLE_API_KEY=") {
					apiKey = strings.TrimPrefix(line, "GOOGLE_API_KEY=")
					apiKey = strings.Trim(apiKey, `"'"`)
					break
				}
			}
		}
	}
	return apiKey
}

func TestOCR_Integration(t *testing.T) {
	apiKey := getAPIKey(t)
	if apiKey == "" {
		t.Skip("Skipping integration test: GOOGLE_API_KEY or GEMINI_API_KEY not set")
	}

	client := NewClient(apiKey)

	imagePath := filepath.Join("..", "..", "images", "japanese-text1.jpg")
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		t.Fatalf("Failed to read test image: %v", err)
	}

	ctx := context.Background()
	result, err := client.OCR(ctx, imageData, "image/jpeg")
	if err != nil {
		if strings.Contains(err.Error(), "overloaded") ||
			strings.Contains(err.Error(), "503") ||
			strings.Contains(err.Error(), "429") ||
			strings.Contains(err.Error(), "RESOURCE_EXHAUSTED") ||
			strings.Contains(strings.ToLower(err.Error()), "quota") {
			t.Skipf("Skipping test due to API overload/quota: %v", err)
		}
		t.Fatalf("OCR failed: %v", err)
	}

	if result == nil {
		t.Fatal("OCR returned nil result")
	}

	if result.RawText == "" {
		t.Error("OCR returned empty raw text")
	}

	if result.Language == "" {
		t.Error("OCR returned empty language")
	}

	if result.Language != "ja" && result.Language != "jpn" {
		t.Logf("Warning: Expected language 'ja' or 'jpn', got '%s'", result.Language)
	}

	rawText := result.RawText
	expectedTexts := []string{
		"薬指の標本",
		"77",
		"わたしの前に事務をやっていたのは",
		"靴音",
	}

	foundCount := 0
	for _, expected := range expectedTexts {
		if strings.Contains(rawText, expected) {
			foundCount++
			t.Logf("Found expected text: %s", expected)
		}
	}

	if foundCount == 0 {
		t.Logf("Warning: None of the expected texts were found in OCR result")
		t.Logf("OCR RawText (first 500 chars): %s", rawText[:min(500, len(rawText))])
	}

	if result.StructuredJSON != "" {
		var structured struct {
			RawText string `json:"raw_text"`
			Language string `json:"language"`
		}
		if err := json.Unmarshal([]byte(result.StructuredJSON), &structured); err == nil {
			if structured.RawText != result.RawText {
				t.Error("StructuredJSON raw_text doesn't match RawText field")
			}
		}
	}

	t.Logf("OCR successful: extracted %d characters, language: %s", len(result.RawText), result.Language)
}

func TestOCR_InvalidImage(t *testing.T) {
	apiKey := getAPIKey(t)
	if apiKey == "" {
		t.Skip("Skipping integration test: GOOGLE_API_KEY or GEMINI_API_KEY not set")
	}

	client := NewClient(apiKey)

	invalidImageData := []byte("not a valid image")
	ctx := context.Background()

	_, err := client.OCR(ctx, invalidImageData, "image/jpeg")
	if err == nil {
		t.Error("Expected error for invalid image data, got nil")
	}
}

func TestOCR_ImageFileExists(t *testing.T) {
	imagePath := filepath.Join("..", "..", "images", "japanese-text1.jpg")
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		t.Fatalf("Test image file not found at %s: %v", imagePath, err)
	}

	if len(imageData) == 0 {
		t.Error("Test image file is empty")
	}

	if len(imageData) < 1000 {
		t.Errorf("Test image file seems too small: %d bytes", len(imageData))
	}

	t.Logf("Test image file loaded successfully: %d bytes", len(imageData))
}

func TestOCR_NoAPIKey(t *testing.T) {
	client := NewClient("")
	ctx := context.Background()

	_, err := client.OCR(ctx, []byte("test"), "image/jpeg")
	if err == nil {
		t.Error("Expected error when API key is empty, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("Expected error about client not initialized, got: %v", err)
	}
}

func TestAnnotate_Integration(t *testing.T) {
	apiKey := getAPIKey(t)
	if apiKey == "" {
		t.Skip("Skipping integration test: GOOGLE_API_KEY or GEMINI_API_KEY not set")
	}

	client := NewClient(apiKey)

	ocrText := "薬指の標本\n77\nわたしの前に事務をやっていたのは、どんな人でした?"
	selectedText := "薬指の標本"

	ctx := context.Background()
	result, err := client.Annotate(ctx, ocrText, selectedText)
	if err != nil {
		if strings.Contains(err.Error(), "overloaded") ||
			strings.Contains(err.Error(), "503") ||
			strings.Contains(err.Error(), "429") ||
			strings.Contains(err.Error(), "RESOURCE_EXHAUSTED") ||
			strings.Contains(strings.ToLower(err.Error()), "quota") {
			t.Skipf("Skipping test due to API overload/quota: %v", err)
		}
		t.Fatalf("Annotate failed: %v", err)
	}

	if result == nil {
		t.Fatal("Annotate returned nil result")
	}

	if result.Meaning == "" {
		t.Error("Annotation returned empty meaning")
	}

	if result.UsageExample == "" {
		t.Error("Annotation returned empty usage example")
	}

	if result.WhenToUse == "" {
		t.Error("Annotation returned empty when_to_use")
	}

	if result.WordBreakdown == "" {
		t.Error("Annotation returned empty word breakdown")
	}

	t.Logf("Annotation successful: meaning=%s", result.Meaning)
}

func TestAnnotate_NoAPIKey(t *testing.T) {
	client := NewClient("")
	ctx := context.Background()

	_, err := client.Annotate(ctx, "test ocr text", "test selected")
	if err == nil {
		t.Error("Expected error when API key is empty, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("Expected error about client not initialized, got: %v", err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

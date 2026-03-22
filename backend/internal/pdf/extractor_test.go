package pdf_test

import (
	"testing"

	"github.com/gemini-hackathon/app/internal/pdf"
)

func TestNewExtractor(t *testing.T) {
	e := pdf.NewExtractor()
	if e == nil {
		t.Fatal("expected non-nil extractor")
	}
}

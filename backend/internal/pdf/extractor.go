package pdf

import (
	"fmt"
	"strings"

	ledongpdf "github.com/ledongthuc/pdf"
)

// Extractor provides methods for reading PDF files.
type Extractor interface {
	PageCount(filePath string) (int, error)
	ExtractText(filePath string, pageNumber int) (string, error)
}

type extractor struct{}

// NewExtractor returns a new PDF extractor backed by ledongthuc/pdf.
func NewExtractor() Extractor {
	return &extractor{}
}

func (e *extractor) PageCount(filePath string) (int, error) {
	f, r, err := ledongpdf.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("open PDF: %w", err)
	}
	defer f.Close()
	return r.NumPage(), nil
}

func (e *extractor) ExtractText(filePath string, pageNumber int) (string, error) {
	f, r, err := ledongpdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open PDF: %w", err)
	}
	defer f.Close()

	total := r.NumPage()
	if pageNumber < 1 || pageNumber > total {
		return "", fmt.Errorf("page %d out of range (1-%d)", pageNumber, total)
	}

	page := r.Page(pageNumber)
	if page.V.IsNull() {
		return "", fmt.Errorf("page %d is null", pageNumber)
	}

	texts := page.Content().Text
	var sb strings.Builder
	for _, t := range texts {
		sb.WriteString(t.S)
	}
	return sb.String(), nil
}

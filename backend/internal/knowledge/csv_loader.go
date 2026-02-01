package knowledge

import (
	"encoding/csv"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// csvService implements Service by loading vocabulary from a CSV file.
type csvService struct {
	entries []Entry
	index   map[string]*Entry // index by Kosakata for exact match
}

// NewService creates a new knowledge service from a CSV file.
// The CSV is expected to have headers:
// Kosakata, Kana, Arti (EN / ID), Cara Baca, Deskripsi, Bidang Pekerjaan, Industri, Konteks
func NewService(csvPath string) (Service, error) {
	file, err := os.Open(csvPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Read header row
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Map column names to indices
	colIndex := make(map[string]int)
	for i, col := range header {
		colIndex[strings.TrimSpace(col)] = i
	}

	// Read all rows
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV data: %w", err)
	}

	entries := make([]Entry, 0, len(records))
	index := make(map[string]*Entry)

	for _, row := range records {
		entry := Entry{
			Kosakata:        getColumn(row, colIndex, "Kosakata"),
			Kana:            getColumn(row, colIndex, "Kana"),
			Arti:            getColumn(row, colIndex, "Arti (EN / ID)"),
			CaraBaca:        getColumn(row, colIndex, "Cara Baca"),
			Deskripsi:       getColumn(row, colIndex, "Deskripsi"),
			BidangPekerjaan: parseAndStripLinks(getColumn(row, colIndex, "Bidang Pekerjaan")),
			Industri:        parseAndStripLinks(getColumn(row, colIndex, "Industri")),
			Konteks:         getColumn(row, colIndex, "Konteks"),
		}

		// Skip rows without Kosakata
		if entry.Kosakata == "" {
			continue
		}

		entries = append(entries, entry)
		// Add to index (pointer to the last element)
		index[entry.Kosakata] = &entries[len(entries)-1]
	}

	return &csvService{
		entries: entries,
		index:   index,
	}, nil
}

// NewEmptyService creates an empty knowledge service (no-op).
// This is useful when no CSV file is configured.
func NewEmptyService() Service {
	return &csvService{
		entries: []Entry{},
		index:   make(map[string]*Entry),
	}
}

// Lookup finds entries matching the text.
// It first tries exact match on Kosakata, then substring match.
func (s *csvService) Lookup(text string) []Entry {
	if text == "" {
		return nil
	}

	var results []Entry

	// 1. Try exact match on Kosakata
	if entry, ok := s.index[text]; ok {
		results = append(results, *entry)
	}

	// 2. Try substring match: text contains Kosakata, or Kosakata contains text
	for i := range s.entries {
		entry := &s.entries[i]
		// Skip if already found via exact match
		if entry.Kosakata == text {
			continue
		}

		// Check if the selected text contains this vocabulary term
		if strings.Contains(text, entry.Kosakata) {
			results = append(results, *entry)
			continue
		}

		// Check if the vocabulary term contains the selected text
		if strings.Contains(entry.Kosakata, text) {
			results = append(results, *entry)
		}
	}

	return results
}

// getColumn safely retrieves a column value from a row.
func getColumn(row []string, colIndex map[string]int, colName string) string {
	idx, ok := colIndex[colName]
	if !ok || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

// notionLinkRegex matches Notion-style links: "(https://...)"
var notionLinkRegex = regexp.MustCompile(`\s*\(https?://[^)]+\)`)

// parseAndStripLinks parses a comma-separated field and strips Notion links.
// Example: "Perbankan (https://notion.so/...), Bisnis" -> ["Perbankan", "Bisnis"]
func parseAndStripLinks(field string) []string {
	if field == "" {
		return nil
	}

	parts := strings.Split(field, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		// Strip Notion links
		cleaned := notionLinkRegex.ReplaceAllString(part, "")
		cleaned = strings.TrimSpace(cleaned)
		if cleaned != "" {
			result = append(result, cleaned)
		}
	}

	return result
}

// StripNotionLinks removes Notion-style links from a string.
// Exported for testing purposes.
func StripNotionLinks(field string) string {
	return notionLinkRegex.ReplaceAllString(field, "")
}

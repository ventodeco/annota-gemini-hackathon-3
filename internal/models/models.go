package models

import "time"

type Session struct {
	ID         string
	UserID     *string
	CreatedAt  time.Time
	LastSeenAt time.Time
}

type Scan struct {
	ID        string
	SessionID string
	UserID    *string
	Source    string
	Status    string
	CreatedAt time.Time
}

type ScanImage struct {
	ID          string
	ScanID      string
	StoragePath string
	MimeType    string
	SHA256      *string
	Width       *int
	Height      *int
	CreatedAt   time.Time
}

type OCRResult struct {
	ID             string
	ScanID         string
	Model          string
	Language       *string
	RawText        string
	StructuredJSON *string
	PromptVersion  string
	CreatedAt      time.Time
}

type Annotation struct {
	ID                 string
	ScanID             string
	OCRResultID        string
	SelectedText       string
	SelectionStart     *int
	SelectionEnd       *int
	Meaning            string
	UsageExample       string
	WhenToUse          string
	WordBreakdown      string
	AlternativeMeanings string
	Model              string
	PromptVersion      string
	CreatedAt          time.Time
}

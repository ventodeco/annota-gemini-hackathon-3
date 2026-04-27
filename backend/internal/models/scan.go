package models

import "time"

type Scan struct {
	ID               int64
	UserID           int64
	ImageURL         string
	FullOCRText      *string
	DetectedLanguage *string
	DocumentID       *int64
	PageNumber       *int
	SourceType       string
	Status           string
	FailureReason    *string
	CreatedAt        time.Time
}

package models

import "time"

type Scan struct {
	ID               int64
	UserID           int64
	ImageURL         string
	FullOCRText      *string
	DetectedLanguage *string
	CreatedAt        time.Time
}

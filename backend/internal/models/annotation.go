package models

import "time"

type NuanceData struct {
	Meaning            string `json:"meaning"`
	UsageExample       string `json:"usageExample"`
	UsageTiming        string `json:"usageTiming"`
	WordBreakdown      string `json:"wordBreakdown"`
	AlternativeMeaning string `json:"alternativeMeaning"`
}

type Annotation struct {
	ID              int64
	UserID          int64
	ScanID          *int64
	HighlightedText string
	ContextText     *string
	NuanceData      NuanceData
	IsBookmarked    bool
	CreatedAt       time.Time
}

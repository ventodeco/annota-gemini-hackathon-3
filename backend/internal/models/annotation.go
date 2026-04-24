package models

import "time"

type NuanceData struct {
	Translation           string            `json:"translation,omitempty"`
	ContextualExplanation string            `json:"contextualExplanation,omitempty"`
	UsageExample          string            `json:"usageExample"`
	WhenToUse             string            `json:"whenToUse,omitempty"`
	WordBreakdown         string            `json:"wordBreakdown"`
	AlternativeMeanings   string            `json:"alternativeMeanings,omitempty"`
	Pronunciation         PronunciationData `json:"pronunciation,omitempty"`
	Meaning               string            `json:"meaning"`
	UsageTiming           string            `json:"usageTiming"`
	AlternativeMeaning    string            `json:"alternativeMeaning"`
}

type PronunciationData struct {
	Kana   string `json:"kana,omitempty"`
	Romaji string `json:"romaji,omitempty"`
}

type Annotation struct {
	ID              int64
	UserID          int64
	ScanID          *int64
	SourceType      *string
	DocumentID      *int64
	PageNumber      *int
	SourceLabel     *string
	HighlightedText string
	ContextText     *string
	NuanceData      NuanceData
	IsBookmarked    bool
	CreatedAt       time.Time
}

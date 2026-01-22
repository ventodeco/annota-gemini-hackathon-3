package models

import "time"

type User struct {
	ID                int64
	Email             string
	Provider          string
	ProviderID        string
	AvatarURL         *string
	PreferredLanguage string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

package models

import "time"

type User struct {
	ID                int64
	Email             string
	Provider          string
	ProviderID        string
	AvatarURL         *string
	PreferredLanguage string
	// ClerkUserID maps the local profile to its Clerk subject once linked.
	ClerkUserID *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

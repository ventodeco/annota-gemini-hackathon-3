package models

import "time"

type Document struct {
	ID        int64
	UserID    int64
	FileURL   string
	Filename  string
	PageCount int
	FileSize  int64
	CreatedAt time.Time
}

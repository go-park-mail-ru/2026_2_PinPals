package domain

import "time"

type Pin struct {
	ID          int
	CreatorID   int
	ImageURL    string
	Name        string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type CreatePinInput struct {
	ImageURL    string
	Name        string
	Description *string
}

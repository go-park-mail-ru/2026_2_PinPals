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
	Tags        []string 
}

type UpdatePinInput struct {
	Name        *string
	Description *string
}

type Tag struct {
	ID   int
	Name string
}

package domain

import "time"

type Pin struct {
	ID          int        `json:"pin_id"`
	CreatorID   int        `json:"creator_id"`
	ImageURL    string     `json:"image_url"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	CreatedAt   time.Time  `json:"-"`
	UpdatedAt   time.Time  `json:"-"`
	DeletedAt   *time.Time `json:"-"`
}

type CreatePinInput struct {
	ImageURL    string  `json:"image_url"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

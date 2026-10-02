package model

import "time"

type Pin struct {
	ID          int        `json:"pin_id"`
	CreatorID   int        `json:"creator_id"`
	ImageURL    string     `json:"image_url"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	Deleted     bool       `json:"deleted"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

type CreatePinInput struct {
	ImageURL    string  `json:"image_url"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

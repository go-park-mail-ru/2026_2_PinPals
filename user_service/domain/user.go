package domain

import "time"

type User struct {
	ID          int
	Name        string
	UserTag     string
	BirthDate   time.Time
	Description *string
	AvatarURL   *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type UpdateProfileInput struct {
	Name           *string
	UserTag        *string
	Description    *string
	DescriptionSet bool
	AvatarURL      *string
	AvatarURLSet   bool
}

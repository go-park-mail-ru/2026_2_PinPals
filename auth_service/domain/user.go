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

type RegisterInput struct {
	Name        string
	UserTag     string
	BirthDate   time.Time
	Password    string
	Description *string
	AvatarURL   *string
}

type LoginInput struct {
	UserTag  string
	Password string
}

type AuthResponse struct {
	Token string
	User  User
}

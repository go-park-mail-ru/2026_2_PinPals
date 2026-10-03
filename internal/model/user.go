package model

import "time"

type User struct {
	ID          int        `json:"user_id"`
	Name        string     `json:"name"`
	UserTag     string     `json:"user_tag"`
	BirthDate   time.Time  `json:"birth_date"`
	Description *string    `json:"description,omitempty"`
	AvatarURL   *string    `json:"avatar_url,omitempty"`
	Deleted     bool       `json:"-"`
	CreatedAt   time.Time  `json:"-"`
	UpdatedAt   time.Time  `json:"-"`
	DeletedAt   *time.Time `json:"-"`
}

type RegisterInput struct {
	Name        string    `json:"name"`
	UserTag     string    `json:"user_tag"`
	BirthDate   time.Time `json:"birth_date"`
	Password    string    `json:"password"`
	Description *string   `json:"description,omitempty"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
}

type LoginInput struct {
	UserTag  string `json:"user_tag"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

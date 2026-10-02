package model

import "time"

type User struct {
	ID          int        `json:"user_id"`
	Name        string     `json:"name"`
	UserTag     string     `json:"user_tag"`
	Age         int        `json:"age"`
	Description *string    `json:"description,omitempty"`
	AvatarURL   *string    `json:"avatar_url,omitempty"`
	Deleted     bool       `json:"deleted"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

type RegisterInput struct {
	Name        string  `json:"name"`
	UserTag     string  `json:"user_tag"`
	Age         int     `json:"age"`
	Password    string  `json:"password"`
	Description *string `json:"description,omitempty"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
}

type LoginInput struct {
	UserTag  string `json:"user_tag"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

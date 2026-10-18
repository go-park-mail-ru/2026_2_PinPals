package delivery

import (
	"time"

	"2026_2_PinPals/auth_service/domain"
)

type RegisterRequest struct {
	Name        string    `json:"name"`
	UserTag     string    `json:"user_tag"`
	BirthDate   time.Time `json:"birth_date"`
	Password    string    `json:"password"`
	Description *string   `json:"description,omitempty"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
}

func (r RegisterRequest) ToDomain() domain.RegisterInput {
	return domain.RegisterInput{
		Name:        r.Name,
		UserTag:     r.UserTag,
		BirthDate:   r.BirthDate,
		Password:    r.Password,
		Description: r.Description,
		AvatarURL:   r.AvatarURL,
	}
}

type LoginRequest struct {
	UserTag  string `json:"user_tag"`
	Password string `json:"password"`
}

func (r LoginRequest) ToDomain() domain.LoginInput {
	return domain.LoginInput{
		UserTag:  r.UserTag,
		Password: r.Password,
	}
}

type UserResponse struct {
	ID          int       `json:"user_id"`
	Name        string    `json:"name"`
	UserTag     string    `json:"user_tag"`
	BirthDate   time.Time `json:"birth_date"`
	Description *string   `json:"description,omitempty"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func UserResponseFromDomain(u domain.User) UserResponse {
	return UserResponse{
		ID:          u.ID,
		Name:        u.Name,
		UserTag:     u.UserTag,
		BirthDate:   u.BirthDate,
		Description: u.Description,
		AvatarURL:   u.AvatarURL,
		CreatedAt:   u.CreatedAt,
	}
}

type AuthResponseDTO struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

func AuthResponseFromDomain(r domain.AuthResponse) AuthResponseDTO {
	return AuthResponseDTO{
		Token: r.Token,
		User:  UserResponseFromDomain(r.User),
	}
}

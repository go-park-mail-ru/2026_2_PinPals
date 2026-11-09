package http

import (
	"bytes"
	"encoding/json"
	"time"

	"2026_2_PinPals/user_service/domain"
	"2026_2_PinPals/pkg/images"
)

type UpdateProfileRequest struct {
	Name        *string         `json:"name"`
	UserTag     *string         `json:"user_tag"`
	Description json.RawMessage `json:"description"`
	AvatarURL   json.RawMessage `json:"avatar_url"`
}

func (r UpdateProfileRequest) ToDomain() (domain.UpdateProfileInput, error) {
	input := domain.UpdateProfileInput{Name: r.Name, UserTag: r.UserTag}
	if len(r.Description) > 0 {
		input.DescriptionSet = true
		v, err := decodeNullableString(r.Description)
		if err != nil {
			return input, err
		}
		input.Description = v
	}
	if len(r.AvatarURL) > 0 {
		input.AvatarURLSet = true
		v, err := decodeNullableString(r.AvatarURL)
		if err != nil {
			return input, err
		}
		input.AvatarURL = v
	}
	return input, nil
}

func decodeNullableString(raw json.RawMessage) (*string, error) {
	raw = bytes.TrimSpace(raw)
	if bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return &value, nil
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

func UserResponseFromDomain(u domain.User, baseURL string) UserResponse {
    avatarURL := u.AvatarURL
    if avatarURL != nil {
        publicURL := images.PublicURL(baseURL, *avatarURL)
        avatarURL = &publicURL
    }
    return UserResponse{
        ID:          u.ID,
        Name:        u.Name,
        UserTag:     u.UserTag,
        BirthDate:   u.BirthDate,
        Description: u.Description,
        AvatarURL:   avatarURL,
        CreatedAt:   u.CreatedAt,
    }
}

package http

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"2026_2_PinPals/pkg/images"
	"2026_2_PinPals/user_service/domain"
)

// UpdateProfileRequest contains only profile text fields. Avatar files are
// accepted through multipart/form-data, never as client-supplied URLs.
type UpdateProfileRequest struct {
	Name        *string         `json:"name"`
	UserTag     *string         `json:"user_tag"`
	Description json.RawMessage `json:"description"`
}

func (r UpdateProfileRequest) Validate() error {
	hasChanges := r.Name != nil || r.UserTag != nil || len(r.Description) > 0
	if !hasChanges {
		return domain.ErrValidation
	}
	if r.Name != nil {
		value := strings.TrimSpace(*r.Name)
		if value == "" || utf8.RuneCountInString(value) > 255 {
			return domain.ErrValidation
		}
	}
	if r.UserTag != nil {
		value := strings.TrimSpace(*r.UserTag)
		if value == "" || utf8.RuneCountInString(value) > 64 {
			return domain.ErrValidation
		}
	}
	if len(r.Description) > 0 {
		value, err := decodeDeskNullableString(r.Description)
		if err != nil {
			return domain.ErrValidation
		}
		if value != nil && utf8.RuneCountInString(*value) > 5000 {
			return domain.ErrValidation
		}
	}
	return nil
}

func (r UpdateProfileRequest) ToDomain() (domain.UpdateProfileInput, error) {
	input := domain.UpdateProfileInput{Name: trimStringPointer(r.Name), UserTag: trimStringPointer(r.UserTag)}
	if len(r.Description) > 0 {
		input.DescriptionSet = true
		value, err := decodeDeskNullableString(r.Description)
		if err != nil {
			return input, err
		}
		input.Description = value
	}
	return input, nil
}

func trimStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func decodeDeskNullableString(raw json.RawMessage) (*string, error) {
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

package http

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"

	"2026_2_PinPals/user_service/domain"
)

// UpdateProfileRequest contains only textual profile attributes. Avatar files
// are uploaded separately as multipart/form-data under the `avatar` field.
type UpdateProfileRequest struct {
	Name        *string         `json:"name" validate:"omitempty,min=1,max=255"`
	UserTag     *string         `json:"user_tag" validate:"omitempty,min=1,max=64"`
	Description json.RawMessage `json:"description"`
}

func (r UpdateProfileRequest) Validate(v *validator.Validate) error {
	if err := v.Struct(r); err != nil {
		return err
	}
	if r.Name != nil && strings.TrimSpace(*r.Name) == "" {
		return domain.ErrValidation
	}
	if r.UserTag != nil && strings.TrimSpace(*r.UserTag) == "" {
		return domain.ErrValidation
	}
	if len(r.Description) > 0 {
		value, err := decodeNullableString(r.Description)
		if err != nil {
			return err
		}
		if value != nil && len(*value) > 5000 {
			return domain.ErrValidation
		}
	}
	return nil
}

func (r UpdateProfileRequest) HasChanges() bool {
	return r.Name != nil || r.UserTag != nil || len(r.Description) > 0
}

func (r UpdateProfileRequest) ToDomain() (domain.UpdateProfileInput, error) {
	input := domain.UpdateProfileInput{Name: r.Name, UserTag: r.UserTag}
	if len(r.Description) > 0 {
		input.DescriptionSet = true
		value, err := decodeNullableString(r.Description)
		if err != nil {
			return input, err
		}
		input.Description = value
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

func UserResponseFromDomain(u domain.User) UserResponse {
	return UserResponse{ID: u.ID, Name: u.Name, UserTag: u.UserTag, BirthDate: u.BirthDate, Description: u.Description, AvatarURL: u.AvatarURL, CreatedAt: u.CreatedAt}
}

package http

import (
	"bytes"
	"encoding/json"

	"2026_2_PinPals/user_service/domain"
)

type UpdateProfileRequest struct {
	Name        *string         `json:"name" validate:"required_without_all=UserTag Description AvatarURL"`
	UserTag     *string         `json:"user_tag"`
	Description json.RawMessage `json:"description"`
	AvatarURL   json.RawMessage `json:"avatar_url"`
}

func (r UpdateProfileRequest) ToDomain() (domain.UpdateProfileInput, error) {
	input := domain.UpdateProfileInput{
		Name:    r.Name,
		UserTag: r.UserTag,
	}

	if len(r.Description) > 0 {
		input.DescriptionSet = true

		value, err := decodeNullableString(r.Description)
		if err != nil {
			return input, err
		}

		input.Description = value
	}

	if len(r.AvatarURL) > 0 {
		input.AvatarURLSet = true

		value, err := decodeNullableString(r.AvatarURL)
		if err != nil {
			return input, err
		}

		input.AvatarURL = value
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

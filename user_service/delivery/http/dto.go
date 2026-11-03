package http

import (
    "bytes"
    "encoding/json"
    "errors"

    "2026_2_PinPals/user_service/domain"
)

type UpdateProfileRequest struct {
    Name        json.RawMessage `json:"name" validate:"required_without_all=UserTag Description AvatarURL"`
    UserTag     json.RawMessage `json:"user_tag"`
    Description json.RawMessage `json:"description"`
    AvatarURL   json.RawMessage `json:"avatar_url"`
}

func (r UpdateProfileRequest) ToDomain() (domain.UpdateProfileInput, error) {
    var input domain.UpdateProfileInput
    var err error

    if len(r.Name) > 0 {
        input.Name, err = decodeProfileString(r.Name, false)
        if err != nil {
            return input, err
        }
    }

    if len(r.UserTag) > 0 {
        input.UserTag, err = decodeProfileString(r.UserTag, false)
        if err != nil {
            return input, err
        }
    }

    if len(r.Description) > 0 {
        input.DescriptionSet = true
        input.Description, err = decodeProfileString(r.Description, true)
        if err != nil {
            return input, err
        }
    }

    if len(r.AvatarURL) > 0 {
        input.AvatarURLSet = true
        input.AvatarURL, err = decodeProfileString(r.AvatarURL, true)
        if err != nil {
            return input, err
        }
    }

    return input, nil
}

func decodeProfileString(raw json.RawMessage, allowNull bool) (*string, error) {
    raw = bytes.TrimSpace(raw)

    if bytes.Equal(raw, []byte("null")) {
        if allowNull {
            return nil, nil
        }

        return nil, errors.New("null is not allowed")
    }

    var value string

    if err := json.Unmarshal(raw, &value); err != nil {
        return nil, err
    }

    return &value, nil
}

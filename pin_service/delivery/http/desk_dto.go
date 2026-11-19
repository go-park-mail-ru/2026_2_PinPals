package http

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"2026_2_PinPals/pin_service/domain"
	"2026_2_PinPals/pkg/images"
)

type DeskResponse struct {
	ID           int           `json:"desk_id"`
	CreatorID    int           `json:"creator_id"`
	Name         string        `json:"name"`
	Description  *string       `json:"description,omitempty"`
	AvatarImgURL *string       `json:"avatar_img_url,omitempty"`
	EntryStatus  int16         `json:"entry_status"`
	CreatedAt    time.Time     `json:"created_at"`
	Pins         []PinResponse `json:"pins,omitempty"`
}

func DeskResponseFromDomain(d domain.Desk, pins []domain.Pin, baseURL string) DeskResponse {
	avatarURL := d.AvatarImgURL
	if avatarURL != nil {
		publicURL := images.PublicURL(baseURL, *avatarURL)
		avatarURL = &publicURL
	}
	out := DeskResponse{
		ID: d.ID, CreatorID: d.CreatorID, Name: d.Name,
		Description: d.Description, AvatarImgURL: avatarURL,
		EntryStatus: d.EntryStatus, CreatedAt: d.CreatedAt,
	}
	if pins != nil {
		out.Pins = make([]PinResponse, len(pins))
		for i, pin := range pins {
			out.Pins[i] = PinResponseFromDomain(pin, baseURL)
		}
	}
	return out
}

// Avatar images are uploaded as multipart files. A client may remove an
// existing avatar, but may not assign an arbitrary URL to this field.
type CreateDeskRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	EntryStatus int16   `json:"entry_status"`
}

func (r CreateDeskRequest) Validate() error {
	if utf8.RuneCountInString(r.Name) > 255 || r.EntryStatus < 0 || r.EntryStatus > 2 {
		return domain.ErrValidation
	}
	if r.Description != nil && utf8.RuneCountInString(*r.Description) > 2000 {
		return domain.ErrValidation
	}
	return nil
}

func (r CreateDeskRequest) ToDomain() domain.CreateDeskInput {
	return domain.CreateDeskInput{Name: r.Name, Description: r.Description, EntryStatus: r.EntryStatus}
}

type UpdateDeskRequest struct {
	Name         *string         `json:"name,omitempty"`
	Description  json.RawMessage `json:"description,omitempty"`
	EntryStatus  *int16          `json:"entry_status,omitempty"`
	RemoveAvatar bool            `json:"remove_avatar,omitempty"`
}

func (r UpdateDeskRequest) Validate() error {
	if r.Name == nil && r.EntryStatus == nil && len(r.Description) == 0 && !r.RemoveAvatar {
		return domain.ErrValidation
	}
	if r.Name != nil {
		name := strings.TrimSpace(*r.Name)
		if name == "" || utf8.RuneCountInString(name) > 255 {
			return domain.ErrValidation
		}
	}
	if r.EntryStatus != nil && (*r.EntryStatus < 0 || *r.EntryStatus > 2) {
		return domain.ErrValidation
	}
	if len(r.Description) > 0 {
		value, err := decodeDeskNullableString(r.Description)
		if err != nil || value != nil && utf8.RuneCountInString(*value) > 2000 {
			return domain.ErrValidation
		}
	}
	return nil
}

func (r UpdateDeskRequest) ToDomain() (domain.UpdateDeskInput, error) {
	input := domain.UpdateDeskInput{Name: trimDeskString(r.Name), EntryStatus: r.EntryStatus}
	if len(r.Description) > 0 {
		input.DescriptionSet = true
		value, err := decodeDeskNullableString(r.Description)
		if err != nil {
			return input, err
		}
		input.Description = value
	}
	if r.RemoveAvatar {
		input.AvatarImgURLSet = true
	}
	return input, nil
}

func trimDeskString(value *string) *string {
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

type DeskPreviewResponse struct {
	Pins []PinResponse `json:"pins"`
}

func DeskPreviewResponseFromDomain(pins []domain.Pin, baseURL string) DeskPreviewResponse {
	out := DeskPreviewResponse{Pins: make([]PinResponse, len(pins))}
	for i, pin := range pins {
		out.Pins[i] = PinResponseFromDomain(pin, baseURL)
	}
	return out
}

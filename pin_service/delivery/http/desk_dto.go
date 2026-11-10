package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-playground/validator/v10"

	"2026_2_PinPals/pin_service/domain"
)

// CreateDeskRequest describes the JSON body used to create a desk.
type CreateDeskRequest struct {
	Name         string  `json:"name" validate:"omitempty,max=255"`
	Description  *string `json:"description,omitempty" validate:"omitempty,max=2000"`
	EntryStatus  int16   `json:"entry_status" validate:"oneof=0 1 2"`
	AvatarImgURL *string `json:"avatar_img_url,omitempty" validate:"omitempty,max=255"`
}

func (r CreateDeskRequest) ToDomain() domain.CreateDeskInput {
	return domain.CreateDeskInput{
		Name:         r.Name,
		Description:  r.Description,
		EntryStatus:  r.EntryStatus,
		AvatarImgURL: r.AvatarImgURL,
	}
}

// UpdateDeskRequest preserves the distinction between an omitted nullable
// field and an explicit JSON null (which clears the stored value).
type UpdateDeskRequest struct {
	Name         *string         `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Description  json.RawMessage `json:"description,omitempty"`
	EntryStatus  *int16          `json:"entry_status,omitempty"`
	AvatarImgURL json.RawMessage `json:"avatar_img_url,omitempty"`
}

func (r UpdateDeskRequest) Validate(v *validator.Validate) error {
	if r.Name == nil && r.EntryStatus == nil && len(r.Description) == 0 && len(r.AvatarImgURL) == 0 {
		return errors.New("at least one field must be supplied")
	}
	if r.Name != nil && (strings.TrimSpace(*r.Name) == "" || utf8.RuneCountInString(*r.Name) > 255) {
		return errors.New("name must contain between 1 and 255 characters")
	}
	if v == nil {
		v = validator.New()
	}
	if err := v.Struct(r); err != nil {
		return err
	}
	if r.EntryStatus != nil && (*r.EntryStatus < 0 || *r.EntryStatus > 2) {
		return errors.New("entry_status must be between 0 and 2")
	}
	if err := validateNullableString(r.Description, 2000); err != nil {
		return errors.New("description must be a string of at most 2000 characters or null")
	}
	if err := validateNullableString(r.AvatarImgURL, 255); err != nil {
		return errors.New("avatar_img_url must be a string of at most 255 characters or null")
	}
	return nil
}

func validateNullableString(raw json.RawMessage, maxLen int) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil
	}
	value, err := decodeNullableString(raw)
	if err != nil {
		return err
	}
	if value != nil && utf8.RuneCountInString(*value) > maxLen {
		return errors.New("string value exceeds maximum length")
	}
	return nil
}

func (r UpdateDeskRequest) ToDomain() (domain.UpdateDeskInput, error) {
	input := domain.UpdateDeskInput{
		Name:        r.Name,
		EntryStatus: r.EntryStatus,
	}
	if len(r.Description) > 0 {
		input.DescriptionSet = true
		value, err := decodeNullableString(r.Description)
		if err != nil {
			return input, err
		}
		input.Description = value
	}
	if len(r.AvatarImgURL) > 0 {
		input.AvatarImgURLSet = true
		value, err := decodeNullableString(r.AvatarImgURL)
		if err != nil {
			return input, err
		}
		input.AvatarImgURL = value
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

// DeskResponse is the transport representation returned to the frontend.
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
	response := DeskResponse{
		ID:           d.ID,
		CreatorID:    d.CreatorID,
		Name:         d.Name,
		Description:  d.Description,
		AvatarImgURL: d.AvatarImgURL,
		EntryStatus:  d.EntryStatus,
		CreatedAt:    d.CreatedAt,
	}
	if response.AvatarImgURL != nil && baseURL != "" {
		value := baseURL + "/images/" + url.PathEscape(*response.AvatarImgURL)
		response.AvatarImgURL = &value
	}
	if pins != nil {
		response.Pins = make([]PinResponse, len(pins))
		for i, pin := range pins {
			response.Pins[i] = PinResponseFromDomain(pin, baseURL)
		}
	}
	return response
}

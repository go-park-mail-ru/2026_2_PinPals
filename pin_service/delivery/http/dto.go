package http

import (
    "bytes"
    "encoding/json"
    "net/url"
    "strings"
    "time"
    "unicode/utf8"
	"strconv"

    "2026_2_PinPals/pin_service/domain"
	"2026_2_PinPals/pkg/images"
)

type CreatePinRequest struct {
	Name        string   `json:"name" validate:"required,max=255"`
	Description *string  `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

func (r CreatePinRequest) ToDomain(imageName string) domain.CreatePinInput {
	return domain.CreatePinInput{ImageURL: imageName, Name: r.Name, Description: r.Description, Tags: r.Tags}
}

type UpdatePinRequest struct {
    Name        *string         `json:"name,omitempty"`
    Description json.RawMessage `json:"description,omitempty"`
}

func (r UpdatePinRequest) ToDomain() (
    domain.UpdatePinInput,
    bool,
    error,
) {
    input := domain.UpdatePinInput{
        Name: r.Name,
    }

    if input.Name != nil {
        name := strings.TrimSpace(*input.Name)

        if name == "" || utf8.RuneCountInString(name) > 255 {
            return input, false, domain.ErrValidation
        }

        input.Name = &name
    }

    descriptionSet := len(r.Description) > 0

    if descriptionSet {
        raw := bytes.TrimSpace(r.Description)

        if bytes.Equal(raw, []byte("null")) {
            input.Description = nil
        } else {
            var description string

            if err := json.Unmarshal(raw, &description); err != nil {
                return input, false, domain.ErrValidation
            }

            description = strings.TrimSpace(description)

            if utf8.RuneCountInString(description) > 2000 {
                return input, false, domain.ErrValidation
            }

            input.Description = &description
        }
    }

    if input.Name == nil && !descriptionSet {
        return input, false, domain.ErrValidation
    }

    return input, descriptionSet, nil
}

type CursorRequest struct {
	CreatedAt time.Time `json:"created_at" validate:"required"`
	ID        int       `json:"id" validate:"gte=1"`
}

func (c CursorRequest) ToDomain() domain.Cursor {
	return domain.Cursor{CreatedAt: c.CreatedAt, ID: c.ID}
}

type ListRequest struct {
	Limit  *int           `json:"limit" validate:"omitnil,min=1,max=100"`
	Cursor *CursorRequest `json:"cursor" validate:"omitnil"`
}

func (r ListRequest) ToDomain() (int, *domain.Cursor) {
	limit := 20
	if r.Limit != nil {
		limit = *r.Limit
	}
	var cursor *domain.Cursor
	if r.Cursor != nil {
		c := r.Cursor.ToDomain()
		cursor = &c
	}
	return limit, cursor
}

type AuthorResponse struct {
	ID        int     `json:"user_id"`
	Name      string  `json:"name"`
	UserTag   string  `json:"user_tag"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

type PinResponse struct {
	ID          int             `json:"pin_id"`
	CreatorID   int             `json:"creator_id"`
	ImageURL    string          `json:"image_url"`
	Name        string          `json:"name"`
	Description *string         `json:"description,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	Author      *AuthorResponse `json:"author,omitempty"`
}

func PinResponseFromDomain(p domain.Pin, baseURL string) PinResponse {
	resp := PinResponse{ID: p.ID, CreatorID: p.CreatorID, ImageURL: p.ImageURL, Name: p.Name, Description: p.Description, CreatedAt: p.CreatedAt}
	if baseURL != "" {
		resp.ImageURL = baseURL + "/images/" + url.PathEscape(resp.ImageURL)
	}
	return resp
}

type CursorResponse struct {
	CreatedAt time.Time `json:"created_at"`
	ID        int       `json:"id"`
}

func PinWithAuthorResponseFromDomain(p domain.PinWithAuthor, baseURL string) PinResponse {
	resp := PinResponseFromDomain(p.Pin, baseURL)
	if p.Author.ID < 1 {
		return resp
	}
	author := &AuthorResponse{
		ID:      p.Author.ID,
		Name:    p.Author.Name,
		UserTag: p.Author.UserTag,
	}
	if p.Author.AvatarURL != nil {
		avatarURL := images.PublicURL(baseURL, *p.Author.AvatarURL)
		author.AvatarURL = &avatarURL
	}
	resp.Author = author
	return resp
}

func CursorResponseFromDomain(c domain.Cursor) CursorResponse {
	return CursorResponse{CreatedAt: c.CreatedAt, ID: c.ID}
}

type ListResponse struct {
	Pins       []PinResponse   `json:"pins"`
	NextCursor *CursorResponse `json:"next_cursor,omitempty"`
}

func ListResponseFromDomain(page domain.Page, baseURL string) ListResponse {
	pins := make([]PinResponse, len(page.Pins))
	for i, pin := range page.Pins {
		pins[i] = PinResponseFromDomain(pin, baseURL)
	}
	resp := ListResponse{Pins: pins}
	if page.NextCursor != nil {
		cr := CursorResponseFromDomain(*page.NextCursor)
		resp.NextCursor = &cr
	}
	return resp
}

type SearchRequest struct {
	Query  string
	Limit  int
	Offset int
}

func NewSearchRequest(values url.Values) (SearchRequest, error) {
	req := SearchRequest{Query: strings.TrimSpace(values.Get("q")), Limit: 20}
	if raw := values.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil {
			return req, domain.ErrValidation
		}
		req.Limit = limit
	}
	if raw := values.Get("offset"); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil {
			return req, domain.ErrValidation
		}
		req.Offset = offset
	}
	return req, nil
}

type SearchPinsResponse struct {
	Pins []PinResponse `json:"pins"`
}

func SearchPinsResponseFromDomain(pins []domain.Pin, baseURL string) SearchPinsResponse {
	out := SearchPinsResponse{Pins: make([]PinResponse, len(pins))}
	for i, pin := range pins {
		out.Pins[i] = PinResponseFromDomain(pin, baseURL)
	}
	return out
}

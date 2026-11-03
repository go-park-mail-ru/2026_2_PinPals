package http

import (
	"net/url"
	"time"

	"2026_2_PinPals/pin_service/domain"
)

type CreatePinRequest struct {
	ImageURL    string   `json:"image_url"`
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

func (r CreatePinRequest) ToDomain() domain.CreatePinInput {
	return domain.CreatePinInput{
		ImageURL:    r.ImageURL,
		Name:        r.Name,
		Description: r.Description,
		Tags:        r.Tags,
	}
}

type CursorRequest struct {
    CreatedAt time.Time `json:"created_at" validate:"required"`
    ID        int       `json:"id" validate:"gte=1"`
}

func (c CursorRequest) ToDomain() domain.Cursor {
    return domain.Cursor{
        CreatedAt: c.CreatedAt,
        ID:        c.ID,
    }
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

type PinResponse struct {
	ID          int       `json:"pin_id"`
	CreatorID   int       `json:"creator_id"`
	ImageURL    string    `json:"image_url"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func PinResponseFromDomain(p domain.Pin, baseURL string) PinResponse {
	resp := PinResponse{
		ID:          p.ID,
		CreatorID:   p.CreatorID,
		ImageURL:    p.ImageURL,
		Name:        p.Name,
		Description: p.Description,
		CreatedAt:   p.CreatedAt,
	}

	if baseURL != "" {
		resp.ImageURL = baseURL + "/images/" + url.PathEscape(resp.ImageURL)
	}

	return resp
}

type CursorResponse struct {
	CreatedAt time.Time `json:"created_at"`
	ID        int       `json:"id"`
}

func CursorResponseFromDomain(c domain.Cursor) CursorResponse {
	return CursorResponse{
		CreatedAt: c.CreatedAt,
		ID:        c.ID,
	}
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

	resp := ListResponse{
		Pins: pins,
	}

	if page.NextCursor != nil {
		cr := CursorResponseFromDomain(*page.NextCursor)
		resp.NextCursor = &cr
	}

	return resp
}

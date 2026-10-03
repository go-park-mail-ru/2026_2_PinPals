package pins

import (
	"context"
	"errors"
	"strings"

	"2026_2_PinPals/internal/model"
)

var ErrValidation = errors.New("validation error")

type Service struct {
	repository *Repository
}

func NewService(repository *Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Create(ctx context.Context, creatorID int, input model.CreatePinInput) (model.Pin, error) {
	input.ImageURL = strings.TrimSpace(input.ImageURL)
	input.Name = strings.TrimSpace(input.Name)

	if input.ImageURL == "" || input.Name == "" {
		return model.Pin{}, ErrValidation
	}
	if len(input.Name) > 255 {
		return model.Pin{}, ErrValidation
	}
	return s.repository.Create(ctx, creatorID, input)
}

func (s *Service) List(ctx context.Context, limit int, cursor *Cursor) (Page, error) {
	if limit < 1 || limit > 100 {
		return Page{}, ErrValidation
	}

	result, hasNext, err := s.repository.List(ctx, limit, cursor)
	if err != nil {
		return Page{}, err
	}

	page := Page{
		Pins: result,
	}

	if !hasNext || len(result) == 0 {
		return page, nil
	}

	page.Pins = result[:limit]
	last := page.Pins[len(page.Pins)-1]
	page.NextCursor = &Cursor{
		CreatedAt: last.CreatedAt,
		ID:        last.ID,
	}

	return page, nil
}

func (s *Service) GetByID(ctx context.Context, id int) (model.Pin, error) {
	if id < 1 {
		return model.Pin{}, ErrValidation
	}
	return s.repository.GetByID(ctx, id)
}

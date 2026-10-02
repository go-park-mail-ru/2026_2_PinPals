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
	if strings.TrimSpace(input.ImageURL) == "" || strings.TrimSpace(input.Name) == "" {
		return model.Pin{}, ErrValidation
	}
	if len(input.Name) > 255 {
		return model.Pin{}, ErrValidation
	}
	return s.repository.Create(ctx, creatorID, input)
}

func (s *Service) List(ctx context.Context, limit, offset int) ([]model.Pin, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, ErrValidation
	}
	return s.repository.List(ctx, limit, offset)
}

func (s *Service) GetByID(ctx context.Context, id int) (model.Pin, error) {
	if id < 1 {
		return model.Pin{}, ErrValidation
	}
	return s.repository.GetByID(ctx, id)
}

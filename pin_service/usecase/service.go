package usecase

import (
	"context"
	"path/filepath"
	"strings"

	"2026_2_PinPals/pin_service/domain"
)

func isValidImageName(name string) bool {
	if name == "" {
		return false
	}

	if filepath.Base(name) != name {
		return false
	}

	if strings.ContainsAny(name, `/\`) {
		return false
	}

	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return true
	default:
		return false
	}
}

type PinRepository interface {
	Create(
		ctx context.Context,
		creatorID int,
		input domain.CreatePinInput,
	) (domain.Pin, error)

	List(
		ctx context.Context,
		limit int,
		cursor *domain.Cursor,
	) ([]domain.Pin, bool, error)

	GetByID(
		ctx context.Context,
		id int,
	) (domain.Pin, error)
}

type Service struct {
	repository PinRepository
}

func NewService(repository PinRepository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Create(ctx context.Context, creatorID int, input domain.CreatePinInput) (domain.Pin, error) {
	input.ImageURL = strings.TrimSpace(input.ImageURL)
	input.Name = strings.TrimSpace(input.Name)

	if !isValidImageName(input.ImageURL) {
		return domain.Pin{}, domain.ErrValidation
	}

	if input.Name == "" {
		return domain.Pin{}, domain.ErrValidation
	}
	if len(input.Name) > 255 {
		return domain.Pin{}, domain.ErrValidation
	}
	return s.repository.Create(ctx, creatorID, input)
}

func (s *Service) List(ctx context.Context, limit int, cursor *domain.Cursor) (domain.Page, error) {
	if limit < 1 || limit > 100 {
		return domain.Page{}, domain.ErrValidation
	}

	result, hasNext, err := s.repository.List(ctx, limit, cursor)
	if err != nil {
		return domain.Page{}, err
	}

	page := domain.Page{
		Pins: result,
	}

	if !hasNext || len(result) == 0 {
		return page, nil
	}

	page.Pins = result[:limit]
	last := page.Pins[len(page.Pins)-1]
	page.NextCursor = &domain.Cursor{
		CreatedAt: last.CreatedAt,
		ID:        last.ID,
	}

	return page, nil
}

func (s *Service) GetByID(ctx context.Context, id int) (domain.Pin, error) {
	if id < 1 {
		return domain.Pin{}, domain.ErrValidation
	}
	return s.repository.GetByID(ctx, id)
}

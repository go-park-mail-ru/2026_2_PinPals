package usecase

import (
	"context"
	"path/filepath"
	"strings"
	"unicode/utf8"

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

	Update(
		ctx context.Context,
		pinID int,
		input domain.UpdatePinInput,
		descriptionSet bool,
	) (domain.Pin, error)

	Delete(
		ctx context.Context,
		pinID int,
	) error
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



func (s *Service) Update(
    ctx context.Context,
    userID int,
    pinID int,
    input domain.UpdatePinInput,
    descriptionSet bool,
) (domain.Pin, error) {

    if userID < 1 || pinID < 1 {
        return domain.Pin{}, domain.ErrValidation
    }

    if input.Name == nil && !descriptionSet {
        return domain.Pin{}, domain.ErrValidation
    }

    if input.Name != nil {
        name := strings.TrimSpace(*input.Name)

        if name == "" || utf8.RuneCountInString(name) > 255 {
            return domain.Pin{}, domain.ErrValidation
        }

        input.Name = &name
    }

    if descriptionSet && input.Description != nil {
        description := strings.TrimSpace(*input.Description)

        if utf8.RuneCountInString(description) > 2000 {
            return domain.Pin{}, domain.ErrValidation
        }

        input.Description = &description
    }

    current, err := s.repository.GetByID(ctx, pinID)
    if err != nil {
        return domain.Pin{}, err
    }

    if current.CreatorID != userID {
        return domain.Pin{}, domain.ErrPinForbidden
    }

    return s.repository.Update(
        ctx,
        pinID,
        input,
        descriptionSet,
    )
}

func (s *Service) Delete(
    ctx context.Context,
    userID int,
    pinID int,
) (domain.Pin, error) {

    if userID < 1 || pinID < 1 {
        return domain.Pin{}, domain.ErrValidation
    }

    current, err := s.repository.GetByID(ctx, pinID)
    if err != nil {
        return domain.Pin{}, err
    }

    if current.CreatorID != userID {
        return domain.Pin{}, domain.ErrPinForbidden
    }

    if err := s.repository.Delete(ctx, pinID); err != nil {
        return domain.Pin{}, err
    }

    return current, nil
}

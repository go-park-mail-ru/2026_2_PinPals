package usecase

import (
	"context"
	"strings"
	"unicode/utf8"

	"2026_2_PinPals/pin_service/domain"
)

type SearchRepository interface {
	SearchPins(ctx context.Context, query string, limit int, offset int) ([]domain.Pin, error)
	SearchDesks(ctx context.Context, query string, limit int, offset int) ([]domain.Desk, error)
}

type SearchService struct{ repository SearchRepository }

func NewSearchService(repository SearchRepository) *SearchService {
	return &SearchService{repository: repository}
}

func normalizeSearch(query string, limit, offset int) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" || utf8.RuneCountInString(query) > 100 || limit < 1 || limit > 100 || offset < 0 {
		return "", domain.ErrValidation
	}
	return query, nil
}

func (s *SearchService) SearchPins(ctx context.Context, query string, limit, offset int) ([]domain.Pin, error) {
	query, err := normalizeSearch(query, limit, offset)
	if err != nil {
		return nil, err
	}
	return s.repository.SearchPins(ctx, query, limit, offset)
}

func (s *SearchService) SearchDesks(ctx context.Context, query string, limit, offset int) ([]domain.Desk, error) {
	query, err := normalizeSearch(query, limit, offset)
	if err != nil {
		return nil, err
	}
	return s.repository.SearchDesks(ctx, query, limit, offset)
}

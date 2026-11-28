package usecase

import (
	"context"
	"strings"
	"unicode/utf8"

	"2026_2_PinPals/user_service/domain"
)

type UserSearchRepository interface {
	SearchUsers(ctx context.Context, query string, limit int, offset int) ([]domain.User, error)
}

type SearchService struct{ repository UserSearchRepository }

func NewSearchService(repository UserSearchRepository) *SearchService {
	return &SearchService{repository: repository}
}

func (s *SearchService) SearchUsers(ctx context.Context, query string, limit, offset int) ([]domain.User, error) {
	query = strings.TrimSpace(query)
	if query == "" || utf8.RuneCountInString(query) > 100 || limit < 1 || limit > 100 || offset < 0 {
		return nil, domain.ErrValidation
	}
	return s.repository.SearchUsers(ctx, query, limit, offset)
}

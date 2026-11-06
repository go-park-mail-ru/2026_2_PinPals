package usecase

import (
	"2026_2_PinPals/pin_service/domain"
	"context"
)

type LikeRepository interface {
	AddLike(context.Context, int, int) error
	RemoveLike(context.Context, int, int) error
	GetLikeStats(context.Context, int, int) (domain.LikeStats, error)
}
type LikeService struct{ repository LikeRepository }

func NewLikeService(repository LikeRepository) *LikeService {
	return &LikeService{repository: repository}
}
func (s *LikeService) Add(ctx context.Context, userID, pinID int) error {
	if userID < 1 || pinID < 1 {
		return domain.ErrValidation
	}
	return s.repository.AddLike(ctx, userID, pinID)
}
func (s *LikeService) Remove(ctx context.Context, userID, pinID int) error {
	if userID < 1 || pinID < 1 {
		return domain.ErrValidation
	}
	return s.repository.RemoveLike(ctx, userID, pinID)
}
func (s *LikeService) Stats(ctx context.Context, userID, pinID int) (domain.LikeStats, error) {
	if pinID < 1 {
		return domain.LikeStats{}, domain.ErrValidation
	}
	return s.repository.GetLikeStats(ctx, userID, pinID)
}

package usecase

import (
	"context"

	"2026_2_PinPals/user_service/domain"
)

type SubscriptionRepository interface {
	CreateSubscription(ctx context.Context, subscriberID int, subscribeeID int) error
	DeleteSubscription(ctx context.Context, subscriberID int, subscribeeID int) error
}

type SubscriptionService struct{ repository SubscriptionRepository }

func NewSubscriptionService(repository SubscriptionRepository) *SubscriptionService {
	return &SubscriptionService{repository: repository}
}

func (s *SubscriptionService) Subscribe(ctx context.Context, subscriberID, subscribeeID int) error {
	if subscriberID < 1 || subscribeeID < 1 || subscriberID == subscribeeID {
		return domain.ErrValidation
	}
	return s.repository.CreateSubscription(ctx, subscriberID, subscribeeID)
}

func (s *SubscriptionService) Unsubscribe(ctx context.Context, subscriberID, subscribeeID int) error {
	if subscriberID < 1 || subscribeeID < 1 {
		return domain.ErrValidation
	}
	return s.repository.DeleteSubscription(ctx, subscriberID, subscribeeID)
}

package usecase

import (
	"context"
	"strings"

	"2026_2_PinPals/user_service/domain"
)

type UserRepository interface {
	GetProfile(ctx context.Context, userID int) (*domain.User, error)
	UpdateProfile(ctx context.Context, userID int, input *domain.UpdateProfileInput) (*domain.User, error)
	DeleteProfile(ctx context.Context, userID int) error
}

type Service struct{ repository UserRepository }

func NewService(repository UserRepository) *Service { return &Service{repository: repository} }

func (s *Service) GetProfile(ctx context.Context, userID int) (*domain.User, error) {
	if userID < 1 {
		return nil, domain.ErrValidation
	}
	return s.repository.GetProfile(ctx, userID)
}

func (s *Service) UpdateProfile(ctx context.Context, userID int, input domain.UpdateProfileInput) (*domain.User, error) {
	if userID < 1 || input.Name == nil && input.UserTag == nil && !input.DescriptionSet && !input.AvatarURLSet {
		return nil, domain.ErrValidation
	}
	if input.Name != nil {
		value := strings.TrimSpace(*input.Name)
		if value == "" || len(value) > 255 {
			return nil, domain.ErrValidation
		}
		input.Name = &value
	}
	if input.UserTag != nil {
		value := strings.TrimSpace(*input.UserTag)
		if value == "" || len(value) > 64 {
			return nil, domain.ErrValidation
		}
		input.UserTag = &value
	}
	if input.Description != nil {
		value := strings.TrimSpace(*input.Description)
		input.Description = &value
	}
	if input.AvatarURL != nil {
		value := strings.TrimSpace(*input.AvatarURL)
		input.AvatarURL = &value
	}
	return s.repository.UpdateProfile(ctx, userID, &input)
}

func (s *Service) DeleteProfile(ctx context.Context, userID int) error {
	if userID < 1 {
		return domain.ErrValidation
	}
	return s.repository.DeleteProfile(ctx, userID)
}

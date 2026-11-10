package usecase

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=desk_service.go -destination=mocks/mock_desk_repository.go -package=mocks

import (
	"context"
	"strings"

	"2026_2_PinPals/pin_service/domain"
)

type DeskRepository interface {
	CreateDesk(context.Context, int, domain.CreateDeskInput) (domain.Desk, error)
	ListOwn(context.Context, int) ([]domain.Desk, error)
	ListPublic(context.Context, int, int) ([]domain.Desk, error)
	GetDeskByID(context.Context, int) (domain.Desk, error)
	GetDeskWithPins(context.Context, int) (domain.DeskWithPins, error)
	UpdateDesk(context.Context, int, domain.UpdateDeskInput) (domain.Desk, error)
	DeleteDesk(context.Context, int) error
	AddPin(context.Context, int, int, int) error
	RemovePin(context.Context, int, int, int) error
}

type DeskService struct{ repository DeskRepository }

func NewDeskService(repository DeskRepository) *DeskService {
	return &DeskService{repository: repository}
}

func (s *DeskService) Create(ctx context.Context, userID int, input domain.CreateDeskInput) (domain.Desk, error) {
	if userID < 1 || input.EntryStatus < domain.EntryStatusPublic || input.EntryStatus > domain.EntryStatusPrivate {
		return domain.Desk{}, domain.ErrValidation
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		input.Name = "Untitled desk"
	}
	if len(input.Name) > 255 {
		return domain.Desk{}, domain.ErrValidation
	}
	if input.Description != nil {
		v := strings.TrimSpace(*input.Description)
		input.Description = &v
	}
	return s.repository.CreateDesk(ctx, userID, input)
}

func (s *DeskService) ListOwn(ctx context.Context, userID int) ([]domain.Desk, error) {
	if userID < 1 {
		return nil, domain.ErrValidation
	}
	return s.repository.ListOwn(ctx, userID)
}
func (s *DeskService) ListPublic(ctx context.Context, ownerID int, limit int) ([]domain.Desk, error) {
	if ownerID < 1 || limit < 1 || limit > 100 {
		return nil, domain.ErrValidation
	}
	return s.repository.ListPublic(ctx, ownerID, limit)
}

func (s *DeskService) GetWithPins(ctx context.Context, requesterID, deskID int) (domain.DeskWithPins, error) {
	if deskID < 1 {
		return domain.DeskWithPins{}, domain.ErrValidation
	}
	desk, err := s.repository.GetDeskByID(ctx, deskID)
	if err != nil {
		return domain.DeskWithPins{}, err
	}
	if desk.CreatorID != requesterID && desk.EntryStatus != domain.EntryStatusPublic {
		return domain.DeskWithPins{}, domain.ErrDeskForbidden
	}
	return s.repository.GetDeskWithPins(ctx, deskID)
}

func (s *DeskService) Update(ctx context.Context, userID, deskID int, input domain.UpdateDeskInput) (domain.Desk, error) {
	if userID < 1 || deskID < 1 {
		return domain.Desk{}, domain.ErrValidation
	}
	if input.Name == nil && input.EntryStatus == nil && !input.DescriptionSet && !input.AvatarImgURLSet {
		return domain.Desk{}, domain.ErrValidation
	}
	if input.Name != nil {
		v := strings.TrimSpace(*input.Name)
		if v == "" || len(v) > 255 {
			return domain.Desk{}, domain.ErrValidation
		}
		input.Name = &v
	}
	if input.EntryStatus != nil && (*input.EntryStatus < 0 || *input.EntryStatus > 2) {
		return domain.Desk{}, domain.ErrValidation
	}
	if input.Description != nil {
		v := strings.TrimSpace(*input.Description)
		input.Description = &v
	}
	if input.AvatarImgURL != nil {
		v := strings.TrimSpace(*input.AvatarImgURL)
		input.AvatarImgURL = &v
	}
	desk, err := s.repository.GetDeskByID(ctx, deskID)
	if err != nil {
		return domain.Desk{}, err
	}
	if desk.CreatorID != userID {
		return domain.Desk{}, domain.ErrDeskForbidden
	}
	return s.repository.UpdateDesk(ctx, deskID, input)
}

func (s *DeskService) Delete(ctx context.Context, userID, deskID int) error {
	if userID < 1 || deskID < 1 {
		return domain.ErrValidation
	}
	desk, err := s.repository.GetDeskByID(ctx, deskID)
	if err != nil {
		return err
	}
	if desk.CreatorID != userID {
		return domain.ErrDeskForbidden
	}
	return s.repository.DeleteDesk(ctx, deskID)
}
func (s *DeskService) AddPin(ctx context.Context, userID, deskID, pinID int) error {
	if userID < 1 || deskID < 1 || pinID < 1 {
		return domain.ErrValidation
	}
	return s.repository.AddPin(ctx, userID, deskID, pinID)
}
func (s *DeskService) RemovePin(ctx context.Context, userID, deskID, pinID int) error {
	if userID < 1 || deskID < 1 || pinID < 1 {
		return domain.ErrValidation
	}
	return s.repository.RemovePin(ctx, userID, deskID, pinID)
}

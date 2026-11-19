package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"unicode/utf8"

	"2026_2_PinPals/pin_service/domain"
	"2026_2_PinPals/pkg/images"
)

type DeskRepository interface {
	CreateDesk(context.Context, int, domain.CreateDeskInput) (domain.Desk, error)
	ListOwn(context.Context, int) ([]domain.Desk, error)
	ListPublic(context.Context, int, int) ([]domain.Desk, error)
	GetDeskByID(context.Context, int) (domain.Desk, error)
	GetDeskWithPins(context.Context, int) (domain.DeskWithPins, error)
	GetFirstPins(context.Context, int, int) ([]domain.Pin, error)
	UpdateDesk(context.Context, int, domain.UpdateDeskInput) (domain.Desk, error)
	DeleteDesk(context.Context, int) error
	AddPin(context.Context, int, int, int) error
	RemovePin(context.Context, int, int, int) error
}

// ImageStorage — хранилище файлов изображений (аватары досок).
type ImageStorage interface {
	Save(io.Reader) (string, error)
	Delete(string) error
}

type DeskService struct {
	repository DeskRepository
	images     ImageStorage
}

func NewDeskService(repository DeskRepository, storage ImageStorage) *DeskService {
	return &DeskService{repository: repository, images: storage}
}

// saveAvatar сохраняет файл аватара и переводит ошибки хранилища в доменные.
func (s *DeskService) saveAvatar(avatar io.Reader) (string, error) {
	if s.images == nil {
		return "", errors.New("image storage is not configured")
	}
	filename, err := s.images.Save(avatar)
	switch {
	case err == nil:
		return filename, nil
	case errors.Is(err, images.ErrImageTooLarge):
		return "", domain.ErrImageTooLarge
	case errors.Is(err, images.ErrInvalidImage):
		return "", domain.ErrInvalidImage
	default:
		return "", err
	}
}

// removeReplacedAvatar удаляет файл прежнего аватара, если он был заменён или убран.
func (s *DeskService) removeReplacedAvatar(ctx context.Context, previous, current *string) {
	if s.images == nil || previous == nil || *previous == "" {
		return
	}
	if current != nil && *current == *previous {
		return
	}
	if !images.IsStoredFilename(*previous) {
		return
	}
	if err := s.images.Delete(*previous); err != nil {
		slog.ErrorContext(ctx, "failed to delete old desk avatar", "filename", *previous, "error", err)
	}
}

func (s *DeskService) Create(ctx context.Context, userID int, input domain.CreateDeskInput) (domain.Desk, error) {
	if userID < 1 || input.EntryStatus < domain.EntryStatusPublic || input.EntryStatus > domain.EntryStatusPrivate {
		return domain.Desk{}, domain.ErrValidation
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		input.Name = "Untitled desk"
	}
	if utf8.RuneCountInString(input.Name) > 255 {
		return domain.Desk{}, domain.ErrValidation
	}
	if input.Description != nil {
		v := strings.TrimSpace(*input.Description)
		if utf8.RuneCountInString(v) > 2000 {
			return domain.Desk{}, domain.ErrValidation
		}
		input.Description = &v
	}
	if input.Avatar != nil {
		filename, err := s.saveAvatar(input.Avatar)
		if err != nil {
			return domain.Desk{}, err
		}
		input.AvatarImgURL = &filename
	}
	desk, err := s.repository.CreateDesk(ctx, userID, input)
	if err != nil {
		if input.Avatar != nil && input.AvatarImgURL != nil {
			_ = s.images.Delete(*input.AvatarImgURL)
		}
		return domain.Desk{}, err
	}
	return desk, nil
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

func (s *DeskService) getAccessibleDesk(ctx context.Context, requesterID, deskID int) (domain.Desk, error) {
	if deskID < 1 {
		return domain.Desk{}, domain.ErrValidation
	}
	desk, err := s.repository.GetDeskByID(ctx, deskID)
	if err != nil {
		return domain.Desk{}, err
	}
	if desk.CreatorID != requesterID && desk.EntryStatus != domain.EntryStatusPublic {
		return domain.Desk{}, domain.ErrDeskForbidden
	}
	return desk, nil
}

func (s *DeskService) GetWithPins(ctx context.Context, requesterID, deskID int) (domain.DeskWithPins, error) {
	if _, err := s.getAccessibleDesk(ctx, requesterID, deskID); err != nil {
		return domain.DeskWithPins{}, err
	}
	return s.repository.GetDeskWithPins(ctx, deskID)
}

func (s *DeskService) GetFirstPins(ctx context.Context, requesterID, deskID int) ([]domain.Pin, error) {
	if _, err := s.getAccessibleDesk(ctx, requesterID, deskID); err != nil {
		return nil, err
	}
	return s.repository.GetFirstPins(ctx, deskID, domain.DeskPreviewPinsLimit)
}

func (s *DeskService) Update(ctx context.Context, userID, deskID int, input domain.UpdateDeskInput) (domain.Desk, error) {
	if userID < 1 || deskID < 1 {
		return domain.Desk{}, domain.ErrValidation
	}
	if input.Name == nil && input.EntryStatus == nil && !input.DescriptionSet && !input.AvatarImgURLSet && input.Avatar == nil {
		return domain.Desk{}, domain.ErrValidation
	}
	if input.Name != nil {
		v := strings.TrimSpace(*input.Name)
		if v == "" || utf8.RuneCountInString(v) > 255 {
			return domain.Desk{}, domain.ErrValidation
		}
		input.Name = &v
	}
	if input.EntryStatus != nil && (*input.EntryStatus < 0 || *input.EntryStatus > 2) {
		return domain.Desk{}, domain.ErrValidation
	}
	if input.Description != nil {
		v := strings.TrimSpace(*input.Description)
		if utf8.RuneCountInString(v) > 2000 {
			return domain.Desk{}, domain.ErrValidation
		}
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
	var uploaded string
	if input.Avatar != nil {
		uploaded, err = s.saveAvatar(input.Avatar)
		if err != nil {
			return domain.Desk{}, err
		}
		input.AvatarImgURL = &uploaded
		input.AvatarImgURLSet = true
	}

	updated, err := s.repository.UpdateDesk(ctx, deskID, input)
	if err != nil {
		if uploaded != "" {
			_ = s.images.Delete(uploaded)
		}
		return domain.Desk{}, err
	}
	if input.AvatarImgURLSet {
		s.removeReplacedAvatar(ctx, desk.AvatarImgURL, input.AvatarImgURL)
	}
	return updated, nil
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

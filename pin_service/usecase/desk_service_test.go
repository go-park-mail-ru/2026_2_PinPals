package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"2026_2_PinPals/pin_service/domain"
	repomocks "2026_2_PinPals/pin_service/usecase/mocks"
	"go.uber.org/mock/gomock"
)

func TestDeskCreateNormalizesName(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := repomocks.NewMockDeskRepository(ctrl)
	repo.EXPECT().CreateDesk(gomock.Any(), 5, gomock.Any()).DoAndReturn(
		func(_ context.Context, userID int, input domain.CreateDeskInput) (domain.Desk, error) {
			if input.Name != "Art" {
				t.Fatalf("name=%q", input.Name)
			}
			return domain.Desk{ID: 1, CreatorID: userID, Name: input.Name}, nil
		},
	)
	got, err := NewDeskService(repo).Create(context.Background(), 5, domain.CreateDeskInput{Name: "  Art  ", EntryStatus: domain.EntryStatusPublic})
	if err != nil || got.Name != "Art" {
		t.Fatalf("desk=%+v err=%v", got, err)
	}
}

func TestDeskGetProtectsPrivateDesk(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := repomocks.NewMockDeskRepository(ctrl)
	repo.EXPECT().GetDeskByID(gomock.Any(), 1).Return(domain.Desk{ID: 1, CreatorID: 10, EntryStatus: domain.EntryStatusPrivate}, nil)
	_, err := NewDeskService(repo).GetWithPins(context.Background(), 11, 1)
	if !errors.Is(err, domain.ErrDeskForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestDeskUpdateAndDeleteRequireOwner(t *testing.T) {
	t.Run("update", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := repomocks.NewMockDeskRepository(ctrl)
		repo.EXPECT().GetDeskByID(gomock.Any(), 1).Return(domain.Desk{ID: 1, CreatorID: 10}, nil)
		name := "New"
		_, err := NewDeskService(repo).Update(context.Background(), 11, 1, domain.UpdateDeskInput{Name: &name})
		if !errors.Is(err, domain.ErrDeskForbidden) {
			t.Fatalf("expected forbidden, got %v", err)
		}
	})
	t.Run("delete", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := repomocks.NewMockDeskRepository(ctrl)
		repo.EXPECT().GetDeskByID(gomock.Any(), 1).Return(domain.Desk{ID: 1, CreatorID: 10}, nil)
		if err := NewDeskService(repo).Delete(context.Background(), 11, 1); !errors.Is(err, domain.ErrDeskForbidden) {
			t.Fatalf("expected forbidden, got %v", err)
		}
	})
}

func TestDeskGetPublic(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := repomocks.NewMockDeskRepository(ctrl)
	desk := domain.Desk{ID: 1, CreatorID: 10, EntryStatus: domain.EntryStatusPublic}
	repo.EXPECT().GetDeskByID(gomock.Any(), 1).Return(desk, nil)
	repo.EXPECT().GetDeskWithPins(gomock.Any(), 1).Return(domain.DeskWithPins{Desk: desk}, nil)
	if _, err := NewDeskService(repo).GetWithPins(context.Background(), 0, 1); err != nil {
		t.Fatal(err)
	}
}

func TestDeskServiceRejectsInvalidInputsBeforeRepositoryCall(t *testing.T) {
	s := NewDeskService(nil)
	if _, err := s.Create(context.Background(), 0, domain.CreateDeskInput{}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.ListOwn(context.Background(), 0); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("list own: %v", err)
	}
	if _, err := s.ListPublic(context.Background(), 2, 101); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("list public: %v", err)
	}
	if _, err := s.GetWithPins(context.Background(), 2, 0); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("get: %v", err)
	}
	if _, err := s.Update(context.Background(), 2, 3, domain.UpdateDeskInput{}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("update: %v", err)
	}
	if err := s.Delete(context.Background(), 2, 0); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("delete: %v", err)
	}
	if err := s.AddPin(context.Background(), 0, 3, 4); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("add pin: %v", err)
	}
	if err := s.RemovePin(context.Background(), 2, 0, 4); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("remove pin: %v", err)
	}
	if _, err := s.Create(context.Background(), 2, domain.CreateDeskInput{EntryStatus: 9}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("bad status: %v", err)
	}
	if _, err := s.Create(context.Background(), 2, domain.CreateDeskInput{Name: strings.Repeat("x", 256)}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("long name: %v", err)
	}
}

func TestDeskServicePropagatesRepositoryErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := repomocks.NewMockDeskRepository(ctrl)
	wantErr := errors.New("database unavailable")
	repo.EXPECT().ListOwn(gomock.Any(), 7).Return(nil, wantErr)
	if _, err := NewDeskService(repo).ListOwn(context.Background(), 7); !errors.Is(err, wantErr) {
		t.Fatalf("error=%v", err)
	}
}

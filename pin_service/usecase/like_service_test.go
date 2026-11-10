package usecase

import (
	"context"
	"errors"
	"testing"

	"2026_2_PinPals/pin_service/domain"
	repomocks "2026_2_PinPals/pin_service/usecase/mocks"
	"go.uber.org/mock/gomock"
)

func TestLikeServiceAddAndRemove(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := repomocks.NewMockLikeRepository(ctrl)
	repo.EXPECT().AddLike(gomock.Any(), 3, 8).Return(nil)
	repo.EXPECT().RemoveLike(gomock.Any(), 3, 8).Return(nil)
	s := NewLikeService(repo)
	if err := s.Add(context.Background(), 3, 8); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove(context.Background(), 3, 8); err != nil {
		t.Fatal(err)
	}
}

func TestLikeServiceValidatesIDsBeforeRepositoryCall(t *testing.T) {
	s := NewLikeService(nil)
	if err := s.Add(context.Background(), 0, 2); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("add: %v", err)
	}
	if err := s.Remove(context.Background(), 1, 0); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("remove: %v", err)
	}
	if _, err := s.Stats(context.Background(), 0, 0); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("stats: %v", err)
	}
}

func TestLikeServiceStats(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := repomocks.NewMockLikeRepository(ctrl)
	repo.EXPECT().GetLikeStats(gomock.Any(), 5, 7).Return(domain.LikeStats{Count: 12, LikedByMe: true}, nil)
	got, err := NewLikeService(repo).Stats(context.Background(), 5, 7)
	if err != nil || got.Count != 12 || !got.LikedByMe {
		t.Fatalf("stats=%+v err=%v", got, err)
	}
}

func TestLikeServicePropagatesRepositoryErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := repomocks.NewMockLikeRepository(ctrl)
	wantErr := errors.New("database unavailable")
	repo.EXPECT().AddLike(gomock.Any(), 1, 2).Return(wantErr)
	repo.EXPECT().RemoveLike(gomock.Any(), 1, 2).Return(wantErr)
	repo.EXPECT().GetLikeStats(gomock.Any(), 1, 2).Return(domain.LikeStats{}, wantErr)
	s := NewLikeService(repo)
	if err := s.Add(context.Background(), 1, 2); !errors.Is(err, wantErr) {
		t.Fatalf("add: %v", err)
	}
	if err := s.Remove(context.Background(), 1, 2); !errors.Is(err, wantErr) {
		t.Fatalf("remove: %v", err)
	}
	if _, err := s.Stats(context.Background(), 1, 2); !errors.Is(err, wantErr) {
		t.Fatalf("stats: %v", err)
	}
}

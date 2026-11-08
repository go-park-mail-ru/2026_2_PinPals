package usecase

import (
	"2026_2_PinPals/pin_service/domain"
	"context"
	"errors"
	"testing"
)

type fakeLikeRepository struct {
	err    error
	stats  domain.LikeStats
	called string
}

func (f *fakeLikeRepository) AddLike(context.Context, int, int) error { f.called = "add"; return f.err }
func (f *fakeLikeRepository) RemoveLike(context.Context, int, int) error {
	f.called = "remove"
	return f.err
}
func (f *fakeLikeRepository) GetLikeStats(context.Context, int, int) (domain.LikeStats, error) {
	f.called = "stats"
	return f.stats, f.err
}
func TestLikeServiceAddAndRemove(t *testing.T) {
	r := &fakeLikeRepository{}
	s := NewLikeService(r)
	if err := s.Add(context.Background(), 3, 8); err != nil {
		t.Fatal(err)
	}
	if r.called != "add" {
		t.Fatal(r.called)
	}
	if err := s.Remove(context.Background(), 3, 8); err != nil {
		t.Fatal(err)
	}
	if r.called != "remove" {
		t.Fatal(r.called)
	}
}
func TestLikeServiceValidatesIDs(t *testing.T) {
	s := NewLikeService(&fakeLikeRepository{})
	if err := s.Add(context.Background(), 0, 2); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("got %v", err)
	}
}
func TestLikeServiceStats(t *testing.T) {
	r := &fakeLikeRepository{stats: domain.LikeStats{Count: 12, LikedByMe: true}}
	got, err := NewLikeService(r).Stats(context.Background(), 5, 7)
	if err != nil {
		t.Fatal(err)
	}
	if got.Count != 12 || !got.LikedByMe {
		t.Fatalf("unexpected stats: %#v", got)
	}
}

func TestLikeServiceValidatesRemoveAndStats(t *testing.T) {
	s := NewLikeService(&fakeLikeRepository{})
	if err := s.Remove(context.Background(), 1, 0); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected remove validation error, got %v", err)
	}
	if _, err := s.Stats(context.Background(), 0, 0); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected stats validation error, got %v", err)
	}
}

func TestLikeServicePropagatesRepositoryErrors(t *testing.T) {
	want := errors.New("database unavailable")
	repo := &fakeLikeRepository{err: want}
	s := NewLikeService(repo)
	if err := s.Add(context.Background(), 1, 2); !errors.Is(err, want) {
		t.Fatalf("expected add error %v, got %v", want, err)
	}
	if err := s.Remove(context.Background(), 1, 2); !errors.Is(err, want) {
		t.Fatalf("expected remove error %v, got %v", want, err)
	}
	if _, err := s.Stats(context.Background(), 1, 2); !errors.Is(err, want) {
		t.Fatalf("expected stats error %v, got %v", want, err)
	}
}

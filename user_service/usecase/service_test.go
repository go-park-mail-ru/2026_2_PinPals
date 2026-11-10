package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"2026_2_PinPals/user_service/domain"
)

type fakeRepository struct {
	user                         *domain.User
	getErr, updateErr, deleteErr error
	gotUpdate                    *domain.UpdateProfileInput
	deleted                      bool
}

func (f *fakeRepository) GetProfile(context.Context, int) (*domain.User, error) {
	return f.user, f.getErr
}

func (f *fakeRepository) UpdateProfile(_ context.Context, _ int, in *domain.UpdateProfileInput) (*domain.User, error) {
	f.gotUpdate = in
	return f.user, f.updateErr
}

func (f *fakeRepository) DeleteProfile(context.Context, int) error {
	f.deleted = true
	return f.deleteErr
}

func TestGetProfileValidatesID(t *testing.T) {
	s := NewService(&fakeRepository{user: &domain.User{ID: 4}})
	if _, err := s.GetProfile(context.Background(), 0); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestUpdateProfileTrimsFields(t *testing.T) {
	name := "  Alex "
	tag := " alex "
	repo := &fakeRepository{user: &domain.User{ID: 4}}
	s := NewService(repo)
	_, err := s.UpdateProfile(context.Background(), 4, domain.UpdateProfileInput{Name: &name, UserTag: &tag})
	if err != nil {
		t.Fatal(err)
	}
	if *repo.gotUpdate.Name != "Alex" || *repo.gotUpdate.UserTag != "alex" {
		t.Fatalf("fields not normalized: %#v", repo.gotUpdate)
	}
}

func TestUpdateProfileRejectsEmptyUpdate(t *testing.T) {
	s := NewService(&fakeRepository{})
	if _, err := s.UpdateProfile(context.Background(), 4, domain.UpdateProfileInput{}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestDeleteProfileCallsRepositoryWithFake(t *testing.T) {
	repo := &fakeRepository{}
	s := NewService(repo)
	if err := s.DeleteProfile(context.Background(), 4); err != nil {
		t.Fatal(err)
	}
	if !repo.deleted {
		t.Fatal("repository was not called")
	}
}

func TestGetProfileReturnsRepositoryResult(t *testing.T) {
	want := &domain.User{ID: 3, CreatedAt: time.Now()}
	s := NewService(&fakeRepository{user: want})
	got, err := s.GetProfile(context.Background(), 3)
	if err != nil || got != want {
		t.Fatalf("got %#v, %v", got, err)
	}
}

func TestUpdateProfileRejectsInvalidValuesAndPropagatesError(t *testing.T) {
	s := NewService(&fakeRepository{})
	blank := "   "
	if _, err := s.UpdateProfile(context.Background(), 3, domain.UpdateProfileInput{Name: &blank}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected blank name error, got %v", err)
	}
	longTag := strings.Repeat("a", 65)
	if _, err := s.UpdateProfile(context.Background(), 3, domain.UpdateProfileInput{UserTag: &longTag}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected long user tag error, got %v", err)
	}
	want := errors.New("database unavailable")
	if _, err := NewService(&fakeRepository{updateErr: want}).UpdateProfile(context.Background(), 3, domain.UpdateProfileInput{Name: ptrString("Alex")}); !errors.Is(err, want) {
		t.Fatalf("expected repository error %v, got %v", want, err)
	}
	if err := NewService(&fakeRepository{deleteErr: want}).DeleteProfile(context.Background(), 3); !errors.Is(err, want) {
		t.Fatalf("expected delete error %v, got %v", want, err)
	}
}

func ptrString(value string) *string { return &value }

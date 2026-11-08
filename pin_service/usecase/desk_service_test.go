package usecase

import (
	"2026_2_PinPals/pin_service/domain"
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeDeskRepository struct {
	desk     domain.Desk
	desks    []domain.Desk
	withPins domain.DeskWithPins
	err      error
	called   string
}

func (f *fakeDeskRepository) CreateDesk(_ context.Context, id int, in domain.CreateDeskInput) (domain.Desk, error) {
	f.called = "create"
	f.desk = domain.Desk{ID: 1, CreatorID: id, Name: in.Name, EntryStatus: in.EntryStatus}
	return f.desk, f.err
}
func (f *fakeDeskRepository) ListOwn(context.Context, int) ([]domain.Desk, error) {
	f.called = "own"
	return f.desks, f.err
}
func (f *fakeDeskRepository) ListPublic(context.Context, int, int) ([]domain.Desk, error) {
	f.called = "public"
	return f.desks, f.err
}
func (f *fakeDeskRepository) GetDeskByID(context.Context, int) (domain.Desk, error) {
	f.called = "get"
	return f.desk, f.err
}
func (f *fakeDeskRepository) GetDeskWithPins(context.Context, int) (domain.DeskWithPins, error) {
	f.called = "pins"
	return f.withPins, f.err
}
func (f *fakeDeskRepository) UpdateDesk(context.Context, int, domain.UpdateDeskInput) (domain.Desk, error) {
	f.called = "update"
	return f.desk, f.err
}
func (f *fakeDeskRepository) DeleteDesk(context.Context, int) error {
	f.called = "delete"
	return f.err
}
func (f *fakeDeskRepository) AddPin(context.Context, int, int, int) error {
	f.called = "add-pin"
	return f.err
}
func (f *fakeDeskRepository) RemovePin(context.Context, int, int, int) error {
	f.called = "remove-pin"
	return f.err
}

func TestDeskCreateNormalizesName(t *testing.T) {
	repo := &fakeDeskRepository{}
	s := NewDeskService(repo)
	d, err := s.Create(context.Background(), 5, domain.CreateDeskInput{Name: "  Art  ", EntryStatus: domain.EntryStatusPublic})
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "Art" {
		t.Fatalf("name=%q", d.Name)
	}
}
func TestDeskGetProtectsPrivateDesk(t *testing.T) {
	repo := &fakeDeskRepository{desk: domain.Desk{ID: 1, CreatorID: 10, EntryStatus: domain.EntryStatusPrivate}}
	s := NewDeskService(repo)
	_, err := s.GetWithPins(context.Background(), 11, 1)
	if !errors.Is(err, domain.ErrDeskForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}
func TestDeskUpdateRequiresOwner(t *testing.T) {
	name := "New"
	repo := &fakeDeskRepository{desk: domain.Desk{ID: 1, CreatorID: 10}}
	s := NewDeskService(repo)
	_, err := s.Update(context.Background(), 11, 1, domain.UpdateDeskInput{Name: &name})
	if !errors.Is(err, domain.ErrDeskForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}
func TestDeskDeleteRequiresOwner(t *testing.T) {
	repo := &fakeDeskRepository{desk: domain.Desk{ID: 1, CreatorID: 10}}
	s := NewDeskService(repo)
	err := s.Delete(context.Background(), 11, 1)
	if !errors.Is(err, domain.ErrDeskForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}
func TestDeskGetPublic(t *testing.T) {
	repo := &fakeDeskRepository{desk: domain.Desk{ID: 1, CreatorID: 10, EntryStatus: domain.EntryStatusPublic}, withPins: domain.DeskWithPins{Desk: domain.Desk{ID: 1}}}
	s := NewDeskService(repo)
	_, err := s.GetWithPins(context.Background(), 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if repo.called != "pins" {
		t.Fatalf("unexpected repo call: %s", repo.called)
	}
}

func TestDeskServiceRejectsInvalidInputs(t *testing.T) {
	repo := &fakeDeskRepository{}
	s := NewDeskService(repo)
	if _, err := s.Create(context.Background(), 0, domain.CreateDeskInput{}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected create validation error, got %v", err)
	}
	if _, err := s.ListPublic(context.Background(), 2, 101); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected public list validation error, got %v", err)
	}
	if _, err := s.GetWithPins(context.Background(), 2, 0); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected get validation error, got %v", err)
	}
	if _, err := s.Update(context.Background(), 2, 3, domain.UpdateDeskInput{}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected empty update validation error, got %v", err)
	}
	if err := s.Delete(context.Background(), 2, 0); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected delete validation error, got %v", err)
	}
	if err := s.AddPin(context.Background(), 0, 3, 4); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected add-pin validation error, got %v", err)
	}
	if err := s.RemovePin(context.Background(), 2, 0, 4); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected remove-pin validation error, got %v", err)
	}
	if repo.called != "" {
		t.Fatalf("repository must not be called for invalid inputs, last call=%q", repo.called)
	}
}

func TestDeskServiceRejectsInvalidStatusAndLongName(t *testing.T) {
	s := NewDeskService(&fakeDeskRepository{})
	if _, err := s.Create(context.Background(), 2, domain.CreateDeskInput{EntryStatus: 9}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected invalid status error, got %v", err)
	}
	if _, err := s.Create(context.Background(), 2, domain.CreateDeskInput{Name: strings.Repeat("x", 256)}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected long name error, got %v", err)
	}
}

func TestDeskListOwnDelegatesAndPropagatesErrors(t *testing.T) {
	expected := []domain.Desk{{ID: 1, CreatorID: 7, Name: "Work"}}
	repo := &fakeDeskRepository{desks: expected}
	service := NewDeskService(repo)

	got, err := service.ListOwn(context.Background(), 7)
	if err != nil {
		t.Fatalf("ListOwn() error = %v", err)
	}
	if repo.called != "own" || len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("ListOwn() = %#v, repo call = %q", got, repo.called)
	}

	wantErr := errors.New("database unavailable")
	repo.err = wantErr
	_, err = service.ListOwn(context.Background(), 7)
	if !errors.Is(err, wantErr) {
		t.Fatalf("ListOwn() error = %v, want %v", err, wantErr)
	}
}

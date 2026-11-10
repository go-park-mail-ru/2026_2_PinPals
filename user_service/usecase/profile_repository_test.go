package usecase

import (
	"context"
	"testing"

	"2026_2_PinPals/user_service/domain"
	"2026_2_PinPals/user_service/usecase/mocks"

	"go.uber.org/mock/gomock"
)

func TestGetProfileReturnsRepositoryUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockUserRepository(ctrl)
	repo.EXPECT().GetProfile(gomock.Any(), 42).Return(&domain.User{ID: 42, Name: "Alex"}, nil)

	got, err := NewService(repo).GetProfile(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 42 {
		t.Fatalf("expected ID 42, got %d", got.ID)
	}
}

func TestUpdateProfilePassesInputToRepository(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockUserRepository(ctrl)
	name := "  Alex  "
	repo.EXPECT().UpdateProfile(gomock.Any(), 42, gomock.Any()).DoAndReturn(
		func(_ context.Context, id int, in *domain.UpdateProfileInput) (*domain.User, error) {
			if id != 42 || in == nil || in.Name == nil {
				t.Fatalf("unexpected update args: id=%d input=%+v", id, in)
			}
			if *in.Name != "Alex" {
				t.Fatalf("expected trimmed name, got %q", *in.Name)
			}
			return &domain.User{ID: id, Name: *in.Name}, nil
		},
	)
	got, err := NewService(repo).UpdateProfile(context.Background(), 42, domain.UpdateProfileInput{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Alex" {
		t.Fatalf("unexpected name: %q", got.Name)
	}
}

func TestDeleteProfileCallsRepository(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockUserRepository(ctrl)
	repo.EXPECT().DeleteProfile(gomock.Any(), 42).Return(nil)

	if err := NewService(repo).DeleteProfile(context.Background(), 42); err != nil {
		t.Fatal(err)
	}
}

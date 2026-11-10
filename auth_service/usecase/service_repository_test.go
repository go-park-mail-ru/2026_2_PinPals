package usecase

import (
	"context"
	"testing"
	"time"

	"2026_2_PinPals/auth_service/domain"
	"2026_2_PinPals/auth_service/usecase/mocks"
	"go.uber.org/mock/gomock"
)

func TestRegisterUsesGoMockRepository(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockUserRepository(ctrl)
	input := domain.RegisterInput{
		Name: "Alex", UserTag: "alex", BirthDate: time.Date(2004, 7, 21, 0, 0, 0, 0, time.UTC), Password: "password123",
	}
	repo.EXPECT().CreateUser(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, got domain.RegisterInput, passwordHash string) (domain.User, error) {
			if got.UserTag != "alex" {
				t.Fatalf("unexpected normalized user tag: %q", got.UserTag)
			}
			if passwordHash == "" {
				t.Fatal("password hash must not be empty")
			}
			return domain.User{ID: 1, Name: got.Name, UserTag: got.UserTag}, nil
		},
	)
	got, err := NewService(repo).Register(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 1 {
		t.Fatalf("expected ID 1, got %d", got.ID)
	}
}

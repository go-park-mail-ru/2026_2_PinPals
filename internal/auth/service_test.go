package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"2026_2_PinPals/internal/model"

	"golang.org/x/crypto/bcrypt"
)

type fakeAuthRepository struct {
	user         model.User
	passwordHash string
	createErr    error
	getErr       error
}

func (f *fakeAuthRepository) CreateUser(
	_ context.Context,
	user model.RegisterInput,
	passwordHash string,
) (model.User, error) {
	if f.createErr != nil {
		return model.User{}, f.createErr
	}

	f.user = model.User{
		ID:        1,
		Name:      user.Name,
		UserTag:   user.UserTag,
		BirthDate: user.BirthDate,
	}

	f.passwordHash = passwordHash

	return f.user, nil
}

func (f *fakeAuthRepository) GetUserWithPasswordHash(
	_ context.Context,
	_ string,
) (model.User, string, error) {
	if f.getErr != nil {
		return model.User{}, "", f.getErr
	}

	return f.user, f.passwordHash, nil
}

func TestValidateRegisterInput(t *testing.T) {
	now := time.Date(
		2026,
		10,
		4,
		0,
		0,
		0,
		0,
		time.UTC,
	)

	tests := []struct {
		name    string
		input   model.RegisterInput
		wantErr bool
	}{
		{
			name: "valid",
			input: model.RegisterInput{
				Name:      "Alex",
				UserTag:   "alex",
				BirthDate: time.Date(2004, 7, 21, 0, 0, 0, 0, time.UTC),
				Password:  "password123",
			},
		},
		{
			name: "empty name",
			input: model.RegisterInput{
				Name:      "",
				UserTag:   "alex",
				BirthDate: time.Date(2004, 7, 21, 0, 0, 0, 0, time.UTC),
				Password:  "password123",
			},
			wantErr: true,
		},
		{
			name: "empty user tag",
			input: model.RegisterInput{
				Name:      "Alex",
				UserTag:   "",
				BirthDate: time.Date(2004, 7, 21, 0, 0, 0, 0, time.UTC),
				Password:  "password123",
			},
			wantErr: true,
		},
		{
			name: "under 13",
			input: model.RegisterInput{
				Name:      "Alex",
				UserTag:   "alex",
				BirthDate: time.Date(2014, 10, 5, 0, 0, 0, 0, time.UTC),
				Password:  "password123",
			},
			wantErr: true,
		},
		{
			name: "exactly 13",
			input: model.RegisterInput{
				Name:      "Alex",
				UserTag:   "alex",
				BirthDate: time.Date(2013, 10, 4, 0, 0, 0, 0, time.UTC),
				Password:  "password123",
			},
		},
		{
			name: "birthday tomorrow",
			input: model.RegisterInput{
				Name:      "Alex",
				UserTag:   "alex",
				BirthDate: time.Date(2013, 10, 5, 0, 0, 0, 0, time.UTC),
				Password:  "password123",
			},
			wantErr: true,
		},
		{
			name: "future birth date",
			input: model.RegisterInput{
				Name:      "Alex",
				UserTag:   "alex",
				BirthDate: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
				Password:  "password123",
			},
			wantErr: true,
		},
		{
			name: "short password",
			input: model.RegisterInput{
				Name:      "Alex",
				UserTag:   "alex",
				BirthDate: time.Date(2004, 7, 21, 0, 0, 0, 0, time.UTC),
				Password:  "1234567",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRegisterInput(tt.input, now)

			if tt.wantErr && !errors.Is(err, ErrValidation) {
				t.Fatalf("expected validation error, got %v", err)
			}

			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRegister(t *testing.T) {
	repo := &fakeAuthRepository{}
	service := NewService(repo)

	input := model.RegisterInput{
		Name:      "  Alex  ",
		UserTag:   "  alex  ",
		BirthDate: time.Date(2004, 7, 21, 0, 0, 0, 0, time.UTC),
		Password:  "password123",
	}

	user, err := service.Register(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user.Name != "Alex" {
		t.Fatalf("expected trimmed name, got %q", user.Name)
	}

	if user.UserTag != "alex" {
		t.Fatalf("expected trimmed user tag, got %q", user.UserTag)
	}

	if user.BirthDate != time.Date(2004, 7, 21, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("unexpected birth date: %v", user.BirthDate)
	}

	if repo.passwordHash == "" {
		t.Fatal("password hash is empty")
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(repo.passwordHash),
		[]byte("password123"),
	); err != nil {
		t.Fatalf("password was not hashed correctly: %v", err)
	}
}

func TestRegisterRepositoryError(t *testing.T) {
	expectedErr := errors.New("database error")

	repo := &fakeAuthRepository{
		createErr: expectedErr,
	}

	service := NewService(repo)

	_, err := service.Register(
		context.Background(),
		model.RegisterInput{
			Name:      "Alex",
			UserTag:   "alex",
			BirthDate: time.Date(2004, 7, 21, 0, 0, 0, 0, time.UTC),
			Password:  "password123",
		},
	)

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestLoginSuccess(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword(
		[]byte("password123"),
		bcrypt.DefaultCost,
	)
	if err != nil {
		t.Fatal(err)
	}

	repo := &fakeAuthRepository{
		user: model.User{
			ID:        42,
			Name:      "Alex",
			UserTag:   "alex",
			BirthDate: time.Date(2004, 7, 21, 0, 0, 0, 0, time.UTC),
		},
		passwordHash: string(hash),
	}

	service := NewService(repo)

	var issuedUserID int

	response, err := service.Login(
		context.Background(),
		model.LoginInput{
			UserTag:  " alex ",
			Password: "password123",
		},
		func(userID int) (string, error) {
			issuedUserID = userID
			return "jwt-token", nil
		},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if response.Token != "jwt-token" {
		t.Fatalf("unexpected token: %q", response.Token)
	}

	if response.User.ID != 42 {
		t.Fatalf("unexpected user id: %d", response.User.ID)
	}

	if issuedUserID != 42 {
		t.Fatalf("unexpected issued user id: %d", issuedUserID)
	}
}

func TestLoginInvalidPassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword(
		[]byte("password123"),
		bcrypt.DefaultCost,
	)
	if err != nil {
		t.Fatal(err)
	}

	repo := &fakeAuthRepository{
		user: model.User{
			ID: 42,
		},
		passwordHash: string(hash),
	}

	service := NewService(repo)

	_, err = service.Login(
		context.Background(),
		model.LoginInput{
			UserTag:  "alex",
			Password: "wrong-password",
		},
		func(_ int) (string, error) {
			t.Fatal("token issuer must not be called")
			return "", nil
		},
	)

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}

func TestLoginUserNotFound(t *testing.T) {
	repo := &fakeAuthRepository{
		getErr: ErrUserNotFound,
	}

	service := NewService(repo)

	_, err := service.Login(
		context.Background(),
		model.LoginInput{
			UserTag:  "alex",
			Password: "password123",
		},
		func(_ int) (string, error) {
			t.Fatal("token issuer must not be called")
			return "", nil
		},
	)

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}

func TestLoginRepositoryError(t *testing.T) {
	expectedErr := errors.New("database error")

	repo := &fakeAuthRepository{
		getErr: expectedErr,
	}

	service := NewService(repo)

	_, err := service.Login(
		context.Background(),
		model.LoginInput{
			UserTag:  "alex",
			Password: "password123",
		},
		func(_ int) (string, error) {
			t.Fatal("token issuer must not be called")
			return "", nil
		},
	)

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestLoginTokenError(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword(
		[]byte("password123"),
		bcrypt.DefaultCost,
	)
	if err != nil {
		t.Fatal(err)
	}

	repo := &fakeAuthRepository{
		user:         model.User{ID: 42},
		passwordHash: string(hash),
	}

	service := NewService(repo)

	expectedErr := errors.New("token error")

	_, err = service.Login(
		context.Background(),
		model.LoginInput{
			UserTag:  "alex",
			Password: "password123",
		},
		func(_ int) (string, error) {
			return "", expectedErr
		},
	)

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

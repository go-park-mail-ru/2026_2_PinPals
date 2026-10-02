package auth

import (
	"context"
	"errors"
	"strings"

	"2026_2_PinPals/internal/model"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrValidation = errors.New("validation error")

type Service struct {
	repository *Repository
}

func NewService(repository *Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Register(ctx context.Context, input model.RegisterInput) (model.User, error) {
	if err := validateRegisterInput(input); err != nil {
		return model.User{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, err
	}

	return s.repository.CreateUser(ctx, input, string(hash))
}

func (s *Service) Login(ctx context.Context, input model.LoginInput, issueToken func(userID int) (string, error)) (model.AuthResponse, error) {
	if strings.TrimSpace(input.UserTag) == "" || input.Password == "" {
		return model.AuthResponse{}, ErrValidation
	}

	user, passwordHash, err := s.repository.GetUserWithPasswordHash(ctx, strings.TrimSpace(input.UserTag))
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return model.AuthResponse{}, ErrInvalidCredentials
		}
		return model.AuthResponse{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)); err != nil {
		return model.AuthResponse{}, ErrInvalidCredentials
	}

	token, err := issueToken(user.ID)
	if err != nil {
		return model.AuthResponse{}, err
	}

	return model.AuthResponse{Token: token, User: user}, nil
}

func validateRegisterInput(input model.RegisterInput) error {
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.UserTag) == "" {
		return ErrValidation
	}
	if input.Age <= 12 {
		return ErrValidation
	}
	if len(input.Password) < 8 {
		return ErrValidation
	}
	if len(input.UserTag) > 64 || len(input.Name) > 255 {
		return ErrValidation
	}
	return nil
}

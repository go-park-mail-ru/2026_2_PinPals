package auth

import (
	"context"
	"errors"
	"strings"
	"time"

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
	input.Name = strings.TrimSpace(input.Name)
	input.UserTag = strings.TrimSpace(input.UserTag)

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
	input.UserTag = strings.TrimSpace(input.UserTag)
	if input.UserTag == "" || input.Password == "" {
		return model.AuthResponse{}, ErrValidation
	}

	user, passwordHash, err := s.repository.GetUserWithPasswordHash(ctx, input.UserTag)
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
	if input.Name == "" || input.UserTag == "" {
		return ErrValidation
	}

	if calculateAge(input.BirthDate, time.Now()) < 13 {
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

func calculateAge(birthDate, today time.Time) int {
	birthDate = birthDate.UTC()
	today = today.UTC()

	age := today.Year() - birthDate.Year()

	birthdayThisYear := time.Date(
		today.Year(),
		birthDate.Month(),
		birthDate.Day(),
		0,
		0,
		0,
		0,
		time.UTC,
	)

	if today.Before(birthdayThisYear) {
		age--
	}

	return age
}

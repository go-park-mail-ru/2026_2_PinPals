package usecase

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"

	"2026_2_PinPals/auth_service/domain"

	"golang.org/x/crypto/bcrypt"
)

type UserRepository interface {
	CreateUser(
		ctx context.Context,
		user domain.RegisterInput,
		passwordHash string,
	) (domain.User, error)

	GetUserWithPasswordHash(
		ctx context.Context,
		userTag string,
	) (domain.User, string, error)
}

type Service struct {
	repository UserRepository
}

func NewService(repository UserRepository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Register(ctx context.Context, input domain.RegisterInput) (domain.User, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.UserTag = strings.TrimSpace(input.UserTag)

	if err := validateRegisterInput(input, time.Now()); err != nil {
		return domain.User{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, err
	}

	return s.repository.CreateUser(ctx, input, string(hash))
}

func (s *Service) Login(ctx context.Context, input domain.LoginInput, issueToken func(userID int) (string, error)) (domain.AuthResponse, error) {
	input.UserTag = strings.TrimSpace(input.UserTag)
	if input.UserTag == "" || input.Password == "" {
		return domain.AuthResponse{}, domain.ErrValidation
	}

	user, passwordHash, err := s.repository.GetUserWithPasswordHash(ctx, input.UserTag)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return domain.AuthResponse{}, domain.ErrInvalidCredentials
		}

		return domain.AuthResponse{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)); err != nil {
		return domain.AuthResponse{}, domain.ErrInvalidCredentials
	}

	token, err := issueToken(user.ID)
	if err != nil {
		return domain.AuthResponse{}, err
	}

	return domain.AuthResponse{Token: token, User: user}, nil
}

func validateRegisterInput(input domain.RegisterInput, now time.Time) error {
	fields := make(map[string]string)

	if input.Name == "" {
		fields["name"] = domain.ValidationRequired
	} else if len(input.Name) > 255 {
		fields["name"] = domain.ValidationTooLong
	}

	if input.UserTag == "" {
		fields["user_tag"] = domain.ValidationRequired
	} else if len(input.UserTag) > 64 {
		fields["user_tag"] = domain.ValidationTooLong
	}

	if input.BirthDate.IsZero() {
		fields["birth_date"] = domain.ValidationRequired
	} else {
		birthDate := input.BirthDate.UTC()
		currentTime := now.UTC()

		if birthDate.After(currentTime) {
			fields["birth_date"] = domain.ValidationFutureDate
		} else {
			age := calculateAge(birthDate, currentTime)

			if age < 13 {
				fields["birth_date"] = domain.ValidationUnderage
			}
		}
	}

	if input.Password == "" {
		fields["password"] = domain.ValidationRequired
	} else {
		switch {
		case len(input.Password) < 8:
			fields["password"] = domain.ValidationTooShort
		case !hasLetter(input.Password):
			fields["password"] = domain.ValidationMustContainLetter
		}
	}

	if len(fields) > 0 {
		return &domain.ValidationErrors{Fields: fields}
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

func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

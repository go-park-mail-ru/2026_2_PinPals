package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"2026_2_PinPals/user_service/domain"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v5"
)

var _ DB = pgxmock.PgxPoolIface(nil)

func newMockUserRepository(t *testing.T) (*Repository, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create pgx mock pool: %v", err)
	}
	t.Cleanup(mock.Close)
	return NewRepository(mock), mock
}

func TestRepositoryGetProfileWithPGXMock(t *testing.T) {
	repo, mock := newMockUserRepository(t)
	birthDate := time.Date(2000, 3, 4, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	mock.ExpectQuery(regexp.QuoteMeta(queryGetProfile)).
		WithArgs(31).
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "name", "user_tag", "birth_date", "description", "avatar_url", "created_at", "updated_at", "deleted_at"}).
			AddRow(31, "Alex", "alex", birthDate, "bio", "avatar.png", createdAt, updatedAt, nil))

	user, err := repo.GetProfile(context.Background(), 31)
	if err != nil {
		t.Fatalf("GetProfile returned error: %v", err)
	}
	if user.ID != 31 || user.Name != "Alex" || user.UserTag != "alex" || user.AvatarURL == nil || *user.AvatarURL != "avatar.png" {
		t.Fatalf("unexpected user: %#v", user)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet pgxmock expectations: %v", err)
	}
}

func TestRepositoryGetProfileMapsNoRows(t *testing.T) {
	repo, mock := newMockUserRepository(t)
	mock.ExpectQuery(regexp.QuoteMeta(queryGetProfile)).
		WithArgs(31).
		WillReturnError(pgx.ErrNoRows)

	_, err := repo.GetProfile(context.Background(), 31)
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet pgxmock expectations: %v", err)
	}
}

func TestRepositoryUpdateProfileWithPGXMock(t *testing.T) {
	repo, mock := newMockUserRepository(t)
	name := "Alex Updated"
	input := &domain.UpdateProfileInput{Name: &name}
	query, args := buildUpdateProfileQuery(31, input)
	birthDate := time.Date(2000, 3, 4, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(args...).
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "name", "user_tag", "birth_date", "description", "avatar_url", "created_at"}).
			AddRow(31, name, "alex", birthDate, nil, nil, createdAt))

	user, err := repo.UpdateProfile(context.Background(), 31, input)
	if err != nil {
		t.Fatalf("UpdateProfile returned error: %v", err)
	}
	if user.ID != 31 || user.Name != name {
		t.Fatalf("unexpected updated user: %#v", user)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet pgxmock expectations: %v", err)
	}
}

func TestRepositoryUpdateProfileMapsUniqueUserTagViolation(t *testing.T) {
	repo, mock := newMockUserRepository(t)
	name := "Alex"
	input := &domain.UpdateProfileInput{Name: &name}
	query, args := buildUpdateProfileQuery(31, input)
	pgErr := pgxmock.NewPgError("23505", "duplicate key value violates unique constraint")
	pgErr.ConstraintName = userTagConstraint
	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(args...).
		WillReturnError(pgErr)

	_, err := repo.UpdateProfile(context.Background(), 31, input)
	if !errors.Is(err, domain.ErrUserTagExists) {
		t.Fatalf("expected ErrUserTagExists, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet pgxmock expectations: %v", err)
	}
}

func TestRepositoryDeleteProfileWithPGXMock(t *testing.T) {
	repo, mock := newMockUserRepository(t)
	mock.ExpectQuery(regexp.QuoteMeta(queryDeleteProfile)).
		WithArgs(31).
		WillReturnRows(pgxmock.NewRows([]string{"user_id"}).AddRow(31))

	if err := repo.DeleteProfile(context.Background(), 31); err != nil {
		t.Fatalf("DeleteProfile returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet pgxmock expectations: %v", err)
	}
}

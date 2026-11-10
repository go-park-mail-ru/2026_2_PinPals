package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"2026_2_PinPals/user_service/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v5"
)

func newUserRepositoryMock(t *testing.T) (*Repository, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool(pgxmock.QueryMatcherOption(pgxmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("create pgx mock: %v", err)
	}
	t.Cleanup(mock.Close)
	return &Repository{pool: mock}, mock
}

func userRows() *pgxmock.Rows {
	birth := time.Date(2000, 1, 2, 0, 0, 0, 0, time.UTC)
	created := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	return pgxmock.NewRows([]string{"user_id", "name", "user_tag", "birth_date", "description", "avatar_url", "created_at"}).
		AddRow(7, "Alex", "alex", birth, nil, nil, created)
}

func TestUpdateProfileRepository(t *testing.T) {
	ctx := context.Background()
	name, description := "Alex updated", "new bio"
	input := &domain.UpdateProfileInput{Name: &name, DescriptionSet: true, Description: &description}
	query, args := buildUpdateProfileQuery(7, input)

	t.Run("success", func(t *testing.T) {
		repo, mock := newUserRepositoryMock(t)
		mock.ExpectQuery(query).WithArgs(args...).WillReturnRows(userRows())
		got, err := repo.UpdateProfile(ctx, 7, input)
		if err != nil || got.ID != 7 || got.Name != "Alex" || got.UserTag != "alex" {
			t.Fatalf("UpdateProfile() = %+v, %v", got, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("missing user", func(t *testing.T) {
		repo, mock := newUserRepositoryMock(t)
		mock.ExpectQuery(query).WithArgs(args...).WillReturnError(pgx.ErrNoRows)
		if _, err := repo.UpdateProfile(ctx, 7, input); !errors.Is(err, domain.ErrUserNotFound) {
			t.Fatalf("UpdateProfile() error = %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("duplicate user tag", func(t *testing.T) {
		repo, mock := newUserRepositoryMock(t)
		pgErr := &pgconn.PgError{Code: "23505", ConstraintName: userTagConstraint}
		mock.ExpectQuery(query).WithArgs(args...).WillReturnError(pgErr)
		if _, err := repo.UpdateProfile(ctx, 7, input); !errors.Is(err, domain.ErrUserTagExists) {
			t.Fatalf("UpdateProfile() error = %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("unexpected database error", func(t *testing.T) {
		repo, mock := newUserRepositoryMock(t)
		wantErr := errors.New("database offline")
		mock.ExpectQuery(query).WithArgs(args...).WillReturnError(wantErr)
		if _, err := repo.UpdateProfile(ctx, 7, input); !errors.Is(err, wantErr) {
			t.Fatalf("UpdateProfile() error = %v, want %v", err, wantErr)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestDeleteProfileRepository(t *testing.T) {
	ctx := context.Background()
	t.Run("success", func(t *testing.T) {
		repo, mock := newUserRepositoryMock(t)
		mock.ExpectQuery(queryDeleteProfile).WithArgs(7).
			WillReturnRows(pgxmock.NewRows([]string{"user_id"}).AddRow(7))
		if err := repo.DeleteProfile(ctx, 7); err != nil {
			t.Fatalf("DeleteProfile() error = %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("missing user", func(t *testing.T) {
		repo, mock := newUserRepositoryMock(t)
		mock.ExpectQuery(queryDeleteProfile).WithArgs(7).WillReturnError(pgx.ErrNoRows)
		if err := repo.DeleteProfile(ctx, 7); !errors.Is(err, domain.ErrUserNotFound) {
			t.Fatalf("DeleteProfile() error = %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("unexpected database error", func(t *testing.T) {
		repo, mock := newUserRepositoryMock(t)
		wantErr := errors.New("database offline")
		mock.ExpectQuery(queryDeleteProfile).WithArgs(7).WillReturnError(wantErr)
		if err := repo.DeleteProfile(ctx, 7); !errors.Is(err, wantErr) {
			t.Fatalf("DeleteProfile() error = %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}

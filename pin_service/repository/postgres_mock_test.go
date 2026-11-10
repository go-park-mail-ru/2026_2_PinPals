package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"2026_2_PinPals/pin_service/domain"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v5"
)

func newMockRepository(t *testing.T) (*Repository, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool(pgxmock.QueryMatcherOption(pgxmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("create pgx mock: %v", err)
	}
	t.Cleanup(mock.Close)
	return &Repository{pool: mock}, mock
}

func TestRepositoryGetByIDWithPGXMock(t *testing.T) {
	createdAt := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

	t.Run("success", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryGetPinByID).
			WithArgs(9).
			WillReturnRows(pgxmock.NewRows([]string{
				"pin_id", "creator_id", "image_url", "name", "description", "created_at",
			}).AddRow(9, 7, "beach.jpg", "Beach", nil, createdAt))

		pin, err := repo.GetByID(context.Background(), 9)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if pin.ID != 9 || pin.CreatorID != 7 || pin.Name != "Beach" || pin.ImageURL != "beach.jpg" {
			t.Fatalf("unexpected pin: %+v", pin)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryGetPinByID).WithArgs(404).WillReturnError(pgx.ErrNoRows)

		_, err := repo.GetByID(context.Background(), 404)
		if !errors.Is(err, domain.ErrPinNotFound) {
			t.Fatalf("GetByID() error = %v, want ErrPinNotFound", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}

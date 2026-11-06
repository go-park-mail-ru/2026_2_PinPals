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

var (
	testPinCreatedAt  = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	testDeskCreatedAt = time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
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

func pinRows(pinID, creatorID int, imageURL, name string) *pgxmock.Rows {
	return pgxmock.NewRows([]string{"pin_id", "creator_id", "image_url", "name", "description", "created_at"}).
		AddRow(pinID, creatorID, imageURL, name, nil, testPinCreatedAt)
}

func deskRows(deskID, creatorID int, name string, status int16) *pgxmock.Rows {
	return pgxmock.NewRows([]string{"desk_id", "creator_id", "name", "description", "avatar_img_url", "entry_status", "created_at", "updated_at", "deleted_at"}).
		AddRow(deskID, creatorID, name, nil, nil, status, testDeskCreatedAt, testDeskCreatedAt, nil)
}

func deskPinRows(pinID, creatorID int, imageURL, name string) *pgxmock.Rows {
	return pgxmock.NewRows([]string{"pin_id", "creator_id", "image_url", "name", "description", "created_at", "updated_at", "deleted_at"}).
		AddRow(pinID, creatorID, imageURL, name, nil, testPinCreatedAt, testPinCreatedAt, nil)
}

func assertMockExpectations(t *testing.T, mock pgxmock.PgxPoolIface) {
	t.Helper()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet database expectations: %v", err)
	}
}

func TestRepositoryPinQueries(t *testing.T) {
	t.Run("create with normalized unique tags", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectBegin()
		mock.ExpectQuery(queryCreatePin).
			WithArgs(7, "beach.jpg", "Beach", pgxmock.AnyArg()).
			WillReturnRows(pinRows(11, 7, "beach.jpg", "Beach"))
		mock.ExpectQuery(queryInsertTag).WithArgs("Summer").
			WillReturnRows(pgxmock.NewRows([]string{"tag_id"}).AddRow(31))
		mock.ExpectExec(queryAddTagToPin).WithArgs(11, 31).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectQuery(queryInsertTag).WithArgs("Travel").
			WillReturnRows(pgxmock.NewRows([]string{"tag_id"}).AddRow(32))
		mock.ExpectExec(queryAddTagToPin).WithArgs(11, 32).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectCommit()
		// pgxmock returns the same mock transaction after Commit; its Rollback
		// expectation models the deferred Rollback in Repository.Create.
		mock.ExpectRollback()

		got, err := repo.Create(context.Background(), 7, domain.CreatePinInput{
			ImageURL: "beach.jpg", Name: "Beach", Tags: []string{"Summer", " summer ", " ", "Travel"},
		})
		if err != nil || got.ID != 11 || got.CreatorID != 7 {
			t.Fatalf("Create() = %+v, %v", got, err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("create rolls back on tag insert failure", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		wantErr := errors.New("tag insert failed")
		mock.ExpectBegin()
		mock.ExpectQuery(queryCreatePin).WithArgs(7, "beach.jpg", "Beach", pgxmock.AnyArg()).
			WillReturnRows(pinRows(11, 7, "beach.jpg", "Beach"))
		mock.ExpectQuery(queryInsertTag).WithArgs("Summer").WillReturnError(wantErr)
		mock.ExpectRollback()
		if _, err := repo.Create(context.Background(), 7, domain.CreatePinInput{ImageURL: "beach.jpg", Name: "Beach", Tags: []string{"Summer"}}); !errors.Is(err, wantErr) {
			t.Fatalf("Create() error = %v, want %v", err, wantErr)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("update success", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		newName := "New name"
		mock.ExpectQuery(queryUpdatePin).WithArgs(11, &newName, false, pgxmock.AnyArg()).
			WillReturnRows(pinRows(11, 7, "beach.jpg", "New name"))
		got, err := repo.Update(context.Background(), 11, domain.UpdatePinInput{Name: &newName}, false)
		if err != nil || got.ID != 11 || got.Name != "New name" {
			t.Fatalf("Update() = %+v, %v", got, err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("update maps missing pin", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		newName, description := "New name", "description"
		mock.ExpectQuery(queryUpdatePin).WithArgs(11, &newName, true, &description).
			WillReturnError(pgx.ErrNoRows)
		if _, err := repo.Update(context.Background(), 11, domain.UpdatePinInput{Name: &newName, Description: &description}, true); !errors.Is(err, domain.ErrPinNotFound) {
			t.Fatalf("Update() error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("delete success and missing pin", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryDeletePin).WithArgs(11).
			WillReturnRows(pgxmock.NewRows([]string{"pin_id"}).AddRow(11))
		if err := repo.Delete(context.Background(), 11); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		mock.ExpectQuery(queryDeletePin).WithArgs(12).WillReturnError(pgx.ErrNoRows)
		if err := repo.Delete(context.Background(), 12); !errors.Is(err, domain.ErrPinNotFound) {
			t.Fatalf("Delete() missing error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("list first and cursor pages", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryListPinsFirstPage).WithArgs(3).
			WillReturnRows(pgxmock.NewRows([]string{"pin_id", "creator_id", "image_url", "name", "description", "created_at"}).
				AddRow(3, 7, "3.jpg", "three", nil, testPinCreatedAt).
				AddRow(2, 7, "2.jpg", "two", nil, testPinCreatedAt).
				AddRow(1, 7, "1.jpg", "one", nil, testPinCreatedAt))
		pins, hasNext, err := repo.List(context.Background(), 2, nil)
		if err != nil || !hasNext || len(pins) != 2 || pins[0].ID != 3 {
			t.Fatalf("List(first) = %#v, %v, %v", pins, hasNext, err)
		}

		cursor := &domain.Cursor{CreatedAt: testPinCreatedAt, ID: 2}
		mock.ExpectQuery(queryListPinsAfterCursor).WithArgs(testPinCreatedAt, 2, 3).
			WillReturnRows(pgxmock.NewRows([]string{"pin_id", "creator_id", "image_url", "name", "description", "created_at"}).
				AddRow(1, 7, "1.jpg", "one", nil, testPinCreatedAt))
		pins, hasNext, err = repo.List(context.Background(), 2, cursor)
		if err != nil || hasNext || len(pins) != 1 || pins[0].ID != 1 {
			t.Fatalf("List(cursor) = %#v, %v, %v", pins, hasNext, err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("get by id success and missing", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryGetPinByID).WithArgs(11).WillReturnRows(pinRows(11, 7, "beach.jpg", "Beach"))
		got, err := repo.GetByID(context.Background(), 11)
		if err != nil || got.ID != 11 || got.Name != "Beach" {
			t.Fatalf("GetByID() = %+v, %v", got, err)
		}
		mock.ExpectQuery(queryGetPinByID).WithArgs(99).WillReturnError(pgx.ErrNoRows)
		if _, err := repo.GetByID(context.Background(), 99); !errors.Is(err, domain.ErrPinNotFound) {
			t.Fatalf("GetByID(missing) error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("search tags success", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(querySearchTags).WithArgs("sum", 10).
			WillReturnRows(pgxmock.NewRows([]string{"tag_id", "name"}).AddRow(1, "summer").AddRow(2, "summer-trip"))
		got, err := repo.SearchTags(context.Background(), "sum", 10)
		if err != nil || len(got) != 2 || got[0].Name != "summer" {
			t.Fatalf("SearchTags() = %#v, %v", got, err)
		}
		assertMockExpectations(t, mock)
	})
}

func TestRepositoryTagAndLikeQueries(t *testing.T) {
	t.Run("add tag success", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryAddTagToPin).WithArgs(11, 31).
			WillReturnRows(pgxmock.NewRows([]string{"tag_relation_id"}).AddRow(4))
		if err := repo.AddTag(context.Background(), 11, 31); err != nil {
			t.Fatal(err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("add tag distinguishes missing pin and tag", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryAddTagToPin).WithArgs(11, 31).WillReturnError(pgx.ErrNoRows)
		mock.ExpectQuery(queryPinExists).WithArgs(11).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
		if err := repo.AddTag(context.Background(), 11, 31); !errors.Is(err, domain.ErrPinNotFound) {
			t.Fatalf("missing pin error = %v", err)
		}

		mock.ExpectQuery(queryAddTagToPin).WithArgs(11, 31).WillReturnError(pgx.ErrNoRows)
		mock.ExpectQuery(queryPinExists).WithArgs(11).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectQuery(queryTagExists).WithArgs(31).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
		if err := repo.AddTag(context.Background(), 11, 31); !errors.Is(err, domain.ErrTagNotFound) {
			t.Fatalf("missing tag error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("remove tag success and missing", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryRemoveTagFromPin).WithArgs(11, 31).
			WillReturnRows(pgxmock.NewRows([]string{"tag_relation_id"}).AddRow(4))
		if err := repo.RemoveTag(context.Background(), 11, 31); err != nil {
			t.Fatal(err)
		}
		mock.ExpectQuery(queryRemoveTagFromPin).WithArgs(11, 99).WillReturnError(pgx.ErrNoRows)
		if err := repo.RemoveTag(context.Background(), 11, 99); !errors.Is(err, domain.ErrTagNotFound) {
			t.Fatalf("RemoveTag(missing) error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("add like success and missing pin", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryAddLike).WithArgs(7, 11).
			WillReturnRows(pgxmock.NewRows([]string{"pin_like_id"}).AddRow(5))
		if err := repo.AddLike(context.Background(), 7, 11); err != nil {
			t.Fatal(err)
		}
		mock.ExpectQuery(queryAddLike).WithArgs(7, 99).WillReturnError(pgx.ErrNoRows)
		mock.ExpectQuery(queryPinExists).WithArgs(99).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
		if err := repo.AddLike(context.Background(), 7, 99); !errors.Is(err, domain.ErrPinNotFound) {
			t.Fatalf("AddLike(missing) error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("remove like validation missing pin and success", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		if err := repo.RemoveLike(context.Background(), 0, 11); !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("RemoveLike invalid user error = %v", err)
		}
		mock.ExpectQuery(queryPinExists).WithArgs(99).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
		if err := repo.RemoveLike(context.Background(), 7, 99); !errors.Is(err, domain.ErrPinNotFound) {
			t.Fatalf("RemoveLike missing pin error = %v", err)
		}
		mock.ExpectQuery(queryPinExists).WithArgs(11).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectExec(queryRemoveLike).WithArgs(7, 11).WillReturnResult(pgxmock.NewResult("DELETE", 1))
		if err := repo.RemoveLike(context.Background(), 7, 11); err != nil {
			t.Fatalf("RemoveLike() error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("like stats validation missing pin and success", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		if _, err := repo.GetLikeStats(context.Background(), 7, 0); !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("GetLikeStats invalid pin error = %v", err)
		}
		mock.ExpectQuery(queryPinExists).WithArgs(99).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
		if _, err := repo.GetLikeStats(context.Background(), 7, 99); !errors.Is(err, domain.ErrPinNotFound) {
			t.Fatalf("GetLikeStats missing pin error = %v", err)
		}
		mock.ExpectQuery(queryPinExists).WithArgs(11).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectQuery(queryGetLikeStats).WithArgs(7, 11).
			WillReturnRows(pgxmock.NewRows([]string{"count", "liked_by_me"}).AddRow(3, true))
		got, err := repo.GetLikeStats(context.Background(), 7, 11)
		if err != nil || got.Count != 3 || !got.LikedByMe {
			t.Fatalf("GetLikeStats() = %+v, %v", got, err)
		}
		assertMockExpectations(t, mock)
	})
}

func TestRepositoryDeskQueries(t *testing.T) {
	t.Run("create desk defaults name and scans fields", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryCreateDesk).WithArgs(7, "Untitled desk", pgxmock.AnyArg(), int16(0), pgxmock.AnyArg()).
			WillReturnRows(deskRows(21, 7, "Untitled desk", 0))
		got, err := repo.CreateDesk(context.Background(), 7, domain.CreateDeskInput{})
		if err != nil || got.ID != 21 || got.Name != "Untitled desk" {
			t.Fatalf("CreateDesk() = %+v, %v", got, err)
		}
		if _, err := repo.CreateDesk(context.Background(), 7, domain.CreateDeskInput{EntryStatus: 3}); !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("invalid CreateDesk status error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("list own and public desks", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryListOwnDesks).WithArgs(7).WillReturnRows(deskRows(21, 7, "Own", 2))
		own, err := repo.ListOwn(context.Background(), 7)
		if err != nil || len(own) != 1 || own[0].Name != "Own" {
			t.Fatalf("ListOwn() = %#v, %v", own, err)
		}
		if _, err := repo.ListPublic(context.Background(), 7, 0); !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("invalid ListPublic limit error = %v", err)
		}
		mock.ExpectQuery(queryListPublicDesks).WithArgs(7, 5).WillReturnRows(deskRows(22, 7, "Public", 0))
		public, err := repo.ListPublic(context.Background(), 7, 5)
		if err != nil || len(public) != 1 || public[0].EntryStatus != domain.EntryStatusPublic {
			t.Fatalf("ListPublic() = %#v, %v", public, err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("list own query error", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		wantErr := errors.New("database unavailable")
		mock.ExpectQuery(queryListOwnDesks).WithArgs(7).WillReturnError(wantErr)
		if _, err := repo.ListOwn(context.Background(), 7); !errors.Is(err, wantErr) {
			t.Fatalf("ListOwn() error = %v, want %v", err, wantErr)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("get desk missing and with pins", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryGetDeskByID).WithArgs(21).WillReturnError(pgx.ErrNoRows)
		if _, err := repo.GetDeskByID(context.Background(), 21); !errors.Is(err, domain.ErrDeskNotFound) {
			t.Fatalf("GetDeskByID missing error = %v", err)
		}
		mock.ExpectQuery(queryGetDeskByID).WithArgs(21).WillReturnRows(deskRows(21, 7, "Travel", 0))
		mock.ExpectQuery(queryGetDeskPins).WithArgs(21).WillReturnRows(deskPinRows(11, 7, "beach.jpg", "Beach"))
		got, err := repo.GetDeskWithPins(context.Background(), 21)
		if err != nil || got.Desk.ID != 21 || len(got.Pins) != 1 || got.Pins[0].ID != 11 {
			t.Fatalf("GetDeskWithPins() = %+v, %v", got, err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("update desk success", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		name := "Updated desk"
		input := domain.UpdateDeskInput{Name: &name}
		mock.ExpectQuery(queryUpdateDesk).WithArgs(21, &name, false, pgxmock.AnyArg(), pgxmock.AnyArg(), false, pgxmock.AnyArg()).
			WillReturnRows(deskRows(21, 7, "Updated desk", 0))
		got, err := repo.UpdateDesk(context.Background(), 21, input)
		if err != nil || got.ID != 21 || got.Name != "Updated desk" {
			t.Fatalf("UpdateDesk() = %+v, %v", got, err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("update desk validation and not found", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		invalid := int16(4)
		if _, err := repo.UpdateDesk(context.Background(), 21, domain.UpdateDeskInput{EntryStatus: &invalid}); !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("invalid UpdateDesk status error = %v", err)
		}
		mock.ExpectQuery(queryUpdateDesk).WithArgs(21, pgxmock.AnyArg(), false, pgxmock.AnyArg(), pgxmock.AnyArg(), false, pgxmock.AnyArg()).
			WillReturnError(pgx.ErrNoRows)
		if _, err := repo.UpdateDesk(context.Background(), 21, domain.UpdateDeskInput{}); !errors.Is(err, domain.ErrDeskNotFound) {
			t.Fatalf("UpdateDesk missing error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("delete desk success and missing", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryDeleteDesk).WithArgs(21).WillReturnRows(pgxmock.NewRows([]string{"desk_id"}).AddRow(21))
		if err := repo.DeleteDesk(context.Background(), 21); err != nil {
			t.Fatal(err)
		}
		mock.ExpectQuery(queryDeleteDesk).WithArgs(22).WillReturnError(pgx.ErrNoRows)
		if err := repo.DeleteDesk(context.Background(), 22); !errors.Is(err, domain.ErrDeskNotFound) {
			t.Fatalf("DeleteDesk missing error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("add and remove pin mappings", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryAddPinToDesk).WithArgs(21, 11, 7).
			WillReturnRows(pgxmock.NewRows([]string{"desk_relation_id"}).AddRow(31))
		if err := repo.AddPin(context.Background(), 7, 21, 11); err != nil {
			t.Fatalf("AddPin() error = %v", err)
		}
		mock.ExpectQuery(queryAddPinToDesk).WithArgs(21, 99, 7).WillReturnError(pgx.ErrNoRows)
		mock.ExpectQuery(queryDeskOwnedBy).WithArgs(21, 7).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
		if err := repo.AddPin(context.Background(), 7, 21, 99); !errors.Is(err, domain.ErrDeskNotFound) {
			t.Fatalf("AddPin missing desk error = %v", err)
		}
		mock.ExpectQuery(queryAddPinToDesk).WithArgs(21, 99, 7).WillReturnError(pgx.ErrNoRows)
		mock.ExpectQuery(queryDeskOwnedBy).WithArgs(21, 7).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectQuery(queryPinExists).WithArgs(99).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
		if err := repo.AddPin(context.Background(), 7, 21, 99); !errors.Is(err, domain.ErrPinNotFound) {
			t.Fatalf("AddPin missing pin error = %v", err)
		}
		mock.ExpectQuery(queryRemovePinFromDesk).WithArgs(21, 11, 7).
			WillReturnRows(pgxmock.NewRows([]string{"desk_relation_id"}).AddRow(31))
		if err := repo.RemovePin(context.Background(), 7, 21, 11); err != nil {
			t.Fatalf("RemovePin() error = %v", err)
		}
		mock.ExpectQuery(queryRemovePinFromDesk).WithArgs(21, 99, 7).WillReturnError(pgx.ErrNoRows)
		if err := repo.RemovePin(context.Background(), 7, 21, 99); !errors.Is(err, domain.ErrPinNotFound) {
			t.Fatalf("RemovePin missing pin error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("first pin success and missing", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryGetFirstPinFromDesk).WithArgs(21).WillReturnRows(deskPinRows(11, 7, "beach.jpg", "Beach"))
		got, err := repo.GetFirstPin(context.Background(), 21)
		if err != nil || got.ID != 11 {
			t.Fatalf("GetFirstPin() = %+v, %v", got, err)
		}
		mock.ExpectQuery(queryGetFirstPinFromDesk).WithArgs(22).WillReturnError(pgx.ErrNoRows)
		if _, err := repo.GetFirstPin(context.Background(), 22); !errors.Is(err, domain.ErrPinNotFound) {
			t.Fatalf("GetFirstPin missing error = %v", err)
		}
		assertMockExpectations(t, mock)
	})
}

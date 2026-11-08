package repository

import (
	"context"
	"errors"
	"testing"

	"2026_2_PinPals/pin_service/domain"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v5"
)

func TestRepositoryCreateTransactionErrorPaths(t *testing.T) {
	ctx := context.Background()
	wantErr := errors.New("database operation failed")

	t.Run("begin error", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectBegin().WillReturnError(wantErr)
		if _, err := repo.Create(ctx, 7, domain.CreatePinInput{ImageURL: "a.jpg", Name: "A"}); !errors.Is(err, wantErr) {
			t.Fatalf("Create() error = %v, want %v", err, wantErr)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("pin insert error rolls back", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectBegin()
		mock.ExpectQuery(queryCreatePin).WithArgs(7, "a.jpg", "A", pgxmock.AnyArg()).WillReturnError(wantErr)
		mock.ExpectRollback()
		if _, err := repo.Create(ctx, 7, domain.CreatePinInput{ImageURL: "a.jpg", Name: "A"}); !errors.Is(err, wantErr) {
			t.Fatalf("Create() error = %v, want %v", err, wantErr)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("tag relation insert error rolls back", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectBegin()
		mock.ExpectQuery(queryCreatePin).WithArgs(7, "a.jpg", "A", pgxmock.AnyArg()).WillReturnRows(pinRows(11, 7, "a.jpg", "A"))
		mock.ExpectQuery(queryInsertTag).WithArgs("tag").WillReturnRows(pgxmock.NewRows([]string{"tag_id"}).AddRow(3))
		mock.ExpectExec(queryAddTagToPin).WithArgs(11, 3).WillReturnError(wantErr)
		mock.ExpectRollback()
		if _, err := repo.Create(ctx, 7, domain.CreatePinInput{ImageURL: "a.jpg", Name: "A", Tags: []string{"tag"}}); !errors.Is(err, wantErr) {
			t.Fatalf("Create() error = %v, want %v", err, wantErr)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("commit error is returned", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectBegin()
		mock.ExpectQuery(queryCreatePin).WithArgs(7, "a.jpg", "A", pgxmock.AnyArg()).WillReturnRows(pinRows(11, 7, "a.jpg", "A"))
		mock.ExpectCommit().WillReturnError(wantErr)
		mock.ExpectRollback()
		if _, err := repo.Create(ctx, 7, domain.CreatePinInput{ImageURL: "a.jpg", Name: "A"}); !errors.Is(err, wantErr) {
			t.Fatalf("Create() error = %v, want %v", err, wantErr)
		}
		assertMockExpectations(t, mock)
	})
}

func TestRepositoryPinQueryErrors(t *testing.T) {
	ctx := context.Background()
	wantErr := errors.New("query failed")

	t.Run("update database error", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		name := "Updated"
		mock.ExpectQuery(queryUpdatePin).WithArgs(11, &name, false, pgxmock.AnyArg()).WillReturnError(wantErr)
		if _, err := repo.Update(ctx, 11, domain.UpdatePinInput{Name: &name}, false); !errors.Is(err, wantErr) {
			t.Fatalf("Update() error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("delete database error", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryDeletePin).WithArgs(11).WillReturnError(wantErr)
		if err := repo.Delete(ctx, 11); !errors.Is(err, wantErr) {
			t.Fatalf("Delete() error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("list query error", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryListPinsFirstPage).WithArgs(3).WillReturnError(wantErr)
		if _, _, err := repo.List(ctx, 2, nil); !errors.Is(err, wantErr) {
			t.Fatalf("List() error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("get by id query error", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryGetPinByID).WithArgs(11).WillReturnError(wantErr)
		if _, err := repo.GetByID(ctx, 11); !errors.Is(err, wantErr) {
			t.Fatalf("GetByID() error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("search tags query and scan errors", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(querySearchTags).WithArgs("a", 5).WillReturnError(wantErr)
		if _, err := repo.SearchTags(ctx, "a", 5); !errors.Is(err, wantErr) {
			t.Fatalf("SearchTags() query error = %v", err)
		}
		mock.ExpectQuery(querySearchTags).WithArgs("a", 5).
			WillReturnRows(pgxmock.NewRows([]string{"tag_id", "name"}).AddRow("not-an-int", "tag"))
		if _, err := repo.SearchTags(ctx, "a", 5); err == nil {
			t.Fatal("SearchTags() expected scan error")
		}
		assertMockExpectations(t, mock)
	})
}

func TestRepositoryTagAndLikeDatabaseErrors(t *testing.T) {
	ctx := context.Background()
	wantErr := errors.New("query failed")

	t.Run("add tag errors", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryAddTagToPin).WithArgs(11, 3).WillReturnError(wantErr)
		if err := repo.AddTag(ctx, 11, 3); !errors.Is(err, wantErr) {
			t.Fatalf("AddTag() error = %v", err)
		}
		mock.ExpectQuery(queryAddTagToPin).WithArgs(11, 3).WillReturnError(pgx.ErrNoRows)
		mock.ExpectQuery(queryPinExists).WithArgs(11).WillReturnError(wantErr)
		if err := repo.AddTag(ctx, 11, 3); !errors.Is(err, wantErr) {
			t.Fatalf("AddTag() pin lookup error = %v", err)
		}
		mock.ExpectQuery(queryAddTagToPin).WithArgs(11, 3).WillReturnError(pgx.ErrNoRows)
		mock.ExpectQuery(queryPinExists).WithArgs(11).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectQuery(queryTagExists).WithArgs(3).WillReturnError(wantErr)
		if err := repo.AddTag(ctx, 11, 3); !errors.Is(err, wantErr) {
			t.Fatalf("AddTag() tag lookup error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("remove tag database error", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryRemoveTagFromPin).WithArgs(11, 3).WillReturnError(wantErr)
		if err := repo.RemoveTag(ctx, 11, 3); !errors.Is(err, wantErr) {
			t.Fatalf("RemoveTag() error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("add like database error paths", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryAddLike).WithArgs(7, 11).WillReturnError(wantErr)
		if err := repo.AddLike(ctx, 7, 11); !errors.Is(err, wantErr) {
			t.Fatalf("AddLike() insert error = %v", err)
		}
		mock.ExpectQuery(queryAddLike).WithArgs(7, 11).WillReturnError(pgx.ErrNoRows)
		mock.ExpectQuery(queryPinExists).WithArgs(11).WillReturnError(wantErr)
		if err := repo.AddLike(ctx, 7, 11); !errors.Is(err, wantErr) {
			t.Fatalf("AddLike() pin lookup error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("remove like database errors", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryPinExists).WithArgs(11).WillReturnError(wantErr)
		if err := repo.RemoveLike(ctx, 7, 11); !errors.Is(err, wantErr) {
			t.Fatalf("RemoveLike() lookup error = %v", err)
		}
		mock.ExpectQuery(queryPinExists).WithArgs(11).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectExec(queryRemoveLike).WithArgs(7, 11).WillReturnError(wantErr)
		if err := repo.RemoveLike(ctx, 7, 11); !errors.Is(err, wantErr) {
			t.Fatalf("RemoveLike() delete error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("like stats database errors", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryPinExists).WithArgs(11).WillReturnError(wantErr)
		if _, err := repo.GetLikeStats(ctx, 7, 11); !errors.Is(err, wantErr) {
			t.Fatalf("GetLikeStats() lookup error = %v", err)
		}
		mock.ExpectQuery(queryPinExists).WithArgs(11).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectQuery(queryGetLikeStats).WithArgs(7, 11).WillReturnError(wantErr)
		if _, err := repo.GetLikeStats(ctx, 7, 11); !errors.Is(err, wantErr) {
			t.Fatalf("GetLikeStats() stats error = %v", err)
		}
		assertMockExpectations(t, mock)
	})
}

func TestRepositoryDeskDatabaseErrors(t *testing.T) {
	ctx := context.Background()
	wantErr := errors.New("database offline")

	t.Run("create desk query error", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryCreateDesk).WithArgs(7, "Art", pgxmock.AnyArg(), int16(0), pgxmock.AnyArg()).WillReturnError(wantErr)
		if _, err := repo.CreateDesk(ctx, 7, domain.CreateDeskInput{Name: "Art"}); !errors.Is(err, wantErr) {
			t.Fatalf("CreateDesk() error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("public desk list query error", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryListPublicDesks).WithArgs(7, 5).WillReturnError(wantErr)
		if _, err := repo.ListPublic(ctx, 7, 5); !errors.Is(err, wantErr) {
			t.Fatalf("ListPublic() error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("get desk by id database error", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryGetDeskByID).WithArgs(21).WillReturnError(wantErr)
		if _, err := repo.GetDeskByID(ctx, 21); !errors.Is(err, wantErr) {
			t.Fatalf("GetDeskByID() error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("get desk pins query error", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryGetDeskByID).WithArgs(21).WillReturnRows(deskRows(21, 7, "Travel", 0))
		mock.ExpectQuery(queryGetDeskPins).WithArgs(21).WillReturnError(wantErr)
		if _, err := repo.GetDeskWithPins(ctx, 21); !errors.Is(err, wantErr) {
			t.Fatalf("GetDeskWithPins() error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("update desk database error", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryUpdateDesk).WithArgs(21, pgxmock.AnyArg(), false, pgxmock.AnyArg(), pgxmock.AnyArg(), false, pgxmock.AnyArg()).WillReturnError(wantErr)
		if _, err := repo.UpdateDesk(ctx, 21, domain.UpdateDeskInput{}); !errors.Is(err, wantErr) {
			t.Fatalf("UpdateDesk() error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("delete desk database error", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryDeleteDesk).WithArgs(21).WillReturnError(wantErr)
		if err := repo.DeleteDesk(ctx, 21); !errors.Is(err, wantErr) {
			t.Fatalf("DeleteDesk() error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("add pin database error", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryAddPinToDesk).WithArgs(21, 11, 7).WillReturnError(wantErr)
		if err := repo.AddPin(ctx, 7, 21, 11); !errors.Is(err, wantErr) {
			t.Fatalf("AddPin() error = %v", err)
		}
		mock.ExpectQuery(queryAddPinToDesk).WithArgs(21, 11, 7).WillReturnError(pgx.ErrNoRows)
		mock.ExpectQuery(queryDeskOwnedBy).WithArgs(21, 7).WillReturnError(wantErr)
		if err := repo.AddPin(ctx, 7, 21, 11); !errors.Is(err, wantErr) {
			t.Fatalf("AddPin() owner lookup error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("remove pin database error", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryRemovePinFromDesk).WithArgs(21, 11, 7).WillReturnError(wantErr)
		if err := repo.RemovePin(ctx, 7, 21, 11); !errors.Is(err, wantErr) {
			t.Fatalf("RemovePin() error = %v", err)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("get first pin database error", func(t *testing.T) {
		repo, mock := newMockRepository(t)
		mock.ExpectQuery(queryGetFirstPinFromDesk).WithArgs(21).WillReturnError(wantErr)
		if _, err := repo.GetFirstPin(ctx, 21); !errors.Is(err, wantErr) {
			t.Fatalf("GetFirstPin() error = %v", err)
		}
		assertMockExpectations(t, mock)
	})
}

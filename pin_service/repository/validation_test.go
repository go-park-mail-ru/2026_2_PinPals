package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"2026_2_PinPals/pin_service/domain"
)

func TestBuildListQuery(t *testing.T) {
	query, args := buildListQuery(nil, 20)
	if query != queryListPinsFirstPage || len(args) != 1 || args[0] != 21 {
		t.Fatalf("first page query=%q args=%#v", query, args)
	}

	cursor := &domain.Cursor{CreatedAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC), ID: 17}
	query, args = buildListQuery(cursor, 10)
	if query != queryListPinsAfterCursor || len(args) != 3 || args[0] != cursor.CreatedAt || args[1] != 17 || args[2] != 11 {
		t.Fatalf("cursor query=%q args=%#v", query, args)
	}
}

func TestRepositoryRejectsInvalidInputsBeforeDatabaseAccess(t *testing.T) {
	repo := &Repository{}
	ctx := context.Background()

	if _, err := repo.CreateDesk(ctx, 1, domain.CreateDeskInput{EntryStatus: 3}); err != domain.ErrValidation {
		t.Fatalf("CreateDesk error=%v", err)
	}
	if _, err := repo.ListPublic(ctx, 1, 0); err != domain.ErrValidation {
		t.Fatalf("ListPublic error=%v", err)
	}
	invalidStatus := int16(-1)
	if _, err := repo.UpdateDesk(ctx, 1, domain.UpdateDeskInput{EntryStatus: &invalidStatus}); err != domain.ErrValidation {
		t.Fatalf("UpdateDesk error=%v", err)
	}
	if err := repo.RemoveLike(ctx, 0, 1); err != domain.ErrValidation {
		t.Fatalf("RemoveLike invalid user error=%v", err)
	}
	if err := repo.RemoveLike(ctx, 1, 0); err != domain.ErrValidation {
		t.Fatalf("RemoveLike invalid pin error=%v", err)
	}
	if _, err := repo.GetLikeStats(ctx, 1, 0); err != domain.ErrValidation {
		t.Fatalf("GetLikeStats error=%v", err)
	}
}

func TestRemovePinQueryHasSingleReturningClause(t *testing.T) {
	const expected = "RETURNING pdr.desk_relation_id"
	count := strings.Count(queryRemovePinFromDesk, expected)
	if count != 1 {
		t.Fatalf("remove-pin SQL must have exactly one RETURNING clause, got %d", count)
	}
}

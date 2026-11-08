package repository

import (
	"strings"
	"testing"

	"2026_2_PinPals/user_service/domain"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestBuildUpdateProfileQuery(t *testing.T) {
	name := "Alex"
	tag := "alex"
	description := "bio"
	avatar := "avatar.jpg"
	query, args := buildUpdateProfileQuery(7, &domain.UpdateProfileInput{
		Name: namePtr(name), UserTag: &tag,
		DescriptionSet: true, Description: &description,
		AvatarURLSet: true, AvatarURL: &avatar,
	})
	for _, fragment := range []string{"name = $2", "user_tag = $3", "description = $4", "avatar_url = $5", "WHERE user_id = $1"} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("query does not contain %q: %s", fragment, query)
		}
	}
	if len(args) != 5 || args[0] != 7 || args[1] != name || args[2] != tag || args[3] != description || args[4] != avatar {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestBuildUpdateProfileQueryNullableFields(t *testing.T) {
	query, args := buildUpdateProfileQuery(3, &domain.UpdateProfileInput{DescriptionSet: true, AvatarURLSet: true})
	if !strings.Contains(query, "description = $2") || !strings.Contains(query, "avatar_url = $3") {
		t.Fatalf("query=%s", query)
	}
	if len(args) != 3 || args[1] != nil || args[2] != nil {
		t.Fatalf("nullable args=%#v", args)
	}
}

func TestIsUserTagUniqueViolation(t *testing.T) {
	if !isUserTagUniqueViolation(&pgconn.PgError{Code: "23505", ConstraintName: userTagConstraint}) {
		t.Fatal("expected matching unique constraint")
	}
	if isUserTagUniqueViolation(&pgconn.PgError{Code: "23505", ConstraintName: "other_constraint"}) {
		t.Fatal("unexpected matching for another constraint")
	}
}

func namePtr(value string) *string { return &value }

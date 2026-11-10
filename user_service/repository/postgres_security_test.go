package repository

import (
	"strings"
	"testing"

	"2026_2_PinPals/user_service/domain"
)

func TestBuildUpdateProfileQueryKeepsUserInputAsArguments(t *testing.T) {
	payload := "x', deleted_at = now() --"
	query, args := buildUpdateProfileQuery(7, &domain.UpdateProfileInput{Name: &payload})
	if strings.Contains(query, payload) {
		t.Fatal("untrusted user input was interpolated into SQL")
	}
	if len(args) != 2 || args[1] != payload {
		t.Fatalf("user input should be a bind argument, got %#v", args)
	}
	if !strings.Contains(query, "name = $2") {
		t.Fatalf("expected fixed column and placeholder, got %s", query)
	}
}

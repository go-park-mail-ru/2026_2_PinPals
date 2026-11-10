package http

import (
	"encoding/json"
	"testing"

	"2026_2_PinPals/user_service/domain"
)

func TestUpdateProfileRequestToDomain(t *testing.T) {
	var req UpdateProfileRequest
	if err := json.Unmarshal([]byte(`{"name":"Alex","user_tag":"alex","description":"hello"}`), &req); err != nil {
		t.Fatal(err)
	}
	if err := req.Validate(); err != nil {
		t.Fatal(err)
	}
	got, err := req.ToDomain()
	if err != nil {
		t.Fatal(err)
	}
	if got.Name == nil || *got.Name != "Alex" || got.UserTag == nil || *got.UserTag != "alex" {
		t.Fatalf("name/user tag mapping failed: %+v", got)
	}
	if !got.DescriptionSet || got.Description == nil || *got.Description != "hello" {
		t.Fatalf("description mapping failed: %+v", got)
	}
	if got.AvatarURLSet {
		t.Fatalf("avatar must only be set through multipart image upload: %+v", got)
	}
}

func TestUpdateProfileRequestEmptyAndInvalidNullableValues(t *testing.T) {
	if err := (UpdateProfileRequest{}).Validate(); err == nil {
		t.Fatal("empty update must be rejected")
	}
	bad := UpdateProfileRequest{Description: json.RawMessage(`{"not":"a string"}`)}
	if _, err := bad.ToDomain(); err == nil {
		t.Fatal("expected object description to be rejected")
	}
	if err := bad.Validate(); err == nil {
		t.Fatal("invalid description must fail validation")
	}
	validClear := UpdateProfileRequest{Description: json.RawMessage(`null`)}
	if err := validClear.Validate(); err != nil {
		t.Fatal(err)
	}
	got, err := validClear.ToDomain()
	if err != nil || !got.DescriptionSet || got.Description != nil {
		t.Fatalf("explicit null should clear description: input=%+v err=%v", got, err)
	}
	if domain.ErrValidation == nil {
		t.Fatal("domain validation sentinel should be initialized")
	}
}

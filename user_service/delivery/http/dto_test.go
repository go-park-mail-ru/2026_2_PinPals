package http

import (
	"encoding/json"
	"testing"
)

func TestUpdateProfileRequestToDomain(t *testing.T) {
	var req UpdateProfileRequest
	if err := json.Unmarshal([]byte(`{"name":"Alex","user_tag":"alex","description":"hello"}`), &req); err != nil {
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
		t.Fatalf("avatar must only be set by the handler from multipart upload: %+v", got)
	}
}

func TestUpdateProfileRequestEmptyAndInvalidNullableValues(t *testing.T) {
	got, err := (UpdateProfileRequest{}).ToDomain()
	if err != nil {
		t.Fatal(err)
	}
	if got.DescriptionSet || got.AvatarURLSet {
		t.Fatalf("omitted nullable fields must remain unset: %+v", got)
	}

	bad := UpdateProfileRequest{Description: json.RawMessage(`{"not":"a string"}`)}
	if _, err := bad.ToDomain(); err == nil {
		t.Fatal("expected object description to be rejected")
	}
}

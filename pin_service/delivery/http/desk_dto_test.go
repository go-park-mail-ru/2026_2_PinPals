package http

import (
	"encoding/json"
	"strings"
	"testing"

	"2026_2_PinPals/pin_service/domain"
	"github.com/go-playground/validator/v10"
)

func TestCreateDeskRequestValidationAndMapping(t *testing.T) {
	v := validator.New()

	valid := CreateDeskRequest{Name: "Travel", EntryStatus: domain.EntryStatusPublic}
	if err := v.Struct(valid); err != nil {
		t.Fatalf("valid create request rejected: %v", err)
	}
	if got := valid.ToDomain(); got.Name != "Travel" || got.EntryStatus != domain.EntryStatusPublic {
		t.Fatalf("unexpected domain input: %+v", got)
	}

	invalid := CreateDeskRequest{Name: strings.Repeat("x", 256), EntryStatus: domain.EntryStatusPublic}
	if err := v.Struct(invalid); err == nil {
		t.Fatal("expected long desk name to be rejected")
	}
	invalid = CreateDeskRequest{Name: "Travel", EntryStatus: 3}
	if err := v.Struct(invalid); err == nil {
		t.Fatal("expected unsupported entry_status to be rejected")
	}
}

func TestUpdateDeskRequestNullableFields(t *testing.T) {
	v := validator.New()

	var request UpdateDeskRequest
	if err := json.Unmarshal([]byte(`{"name":"Updated","description":null,"avatar_img_url":"cover.jpg"}`), &request); err != nil {
		t.Fatal(err)
	}
	if err := request.Validate(v); err != nil {
		t.Fatalf("valid update request rejected: %v", err)
	}
	input, err := request.ToDomain()
	if err != nil {
		t.Fatal(err)
	}
	if input.Name == nil || *input.Name != "Updated" {
		t.Fatalf("unexpected name: %+v", input.Name)
	}
	if !input.DescriptionSet || input.Description != nil {
		t.Fatalf("explicit null description must clear the field: %+v", input)
	}
	if !input.AvatarImgURLSet || input.AvatarImgURL == nil || *input.AvatarImgURL != "cover.jpg" {
		t.Fatalf("avatar mapping is incorrect: %+v", input)
	}
}

func TestUpdateDeskRequestRejectsInvalidBodies(t *testing.T) {
	v := validator.New()
	cases := []UpdateDeskRequest{
		{},
		{Name: stringPointer("")},
		{EntryStatus: int16Pointer(3)},
		{Description: json.RawMessage(`123`)},
		{AvatarImgURL: json.RawMessage(`{"url":"not-a-string"}`)},
		{Description: json.RawMessage(`"` + strings.Repeat("x", 2001) + `"`)},
	}
	for i, request := range cases {
		if err := request.Validate(v); err == nil {
			t.Errorf("case %d: expected validation error", i)
		}
	}
}

func stringPointer(value string) *string { return &value }
func int16Pointer(value int16) *int16    { return &value }

package pins

import (
	"context"
	"errors"
	"testing"
	"time"

	"2026_2_PinPals/internal/model"
)

type fakePinRepository struct {
	createResult model.Pin
	createErr    error

	listResult  []model.Pin
	listHasNext bool
	listErr     error

	getResult model.Pin
	getErr    error
}

func (f *fakePinRepository) Create(
	_ context.Context,
	creatorID int,
	input model.CreatePinInput,
) (model.Pin, error) {
	if f.createErr != nil {
		return model.Pin{}, f.createErr
	}

	return model.Pin{
		ID:          1,
		CreatorID:   creatorID,
		ImageURL:    input.ImageURL,
		Name:        input.Name,
		Description: input.Description,
	}, nil
}

func (f *fakePinRepository) List(
	_ context.Context,
	_ int,
	_ *Cursor,
) ([]model.Pin, bool, error) {
	if f.listErr != nil {
		return nil, false, f.listErr
	}

	return f.listResult, f.listHasNext, nil
}

func (f *fakePinRepository) GetByID(
	_ context.Context,
	_ int,
) (model.Pin, error) {
	if f.getErr != nil {
		return model.Pin{}, f.getErr
	}

	return f.getResult, nil
}

func TestCreatePin(t *testing.T) {
	repo := &fakePinRepository{}
	service := NewService(repo)

	description := "A nice place"

	pin, err := service.Create(
		context.Background(),
		42,
		model.CreatePinInput{
			ImageURL:    " image.jpg ",
			Name:        "  Beach  ",
			Description: &description,
		},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pin.CreatorID != 42 {
		t.Fatalf("expected creator id 42, got %d", pin.CreatorID)
	}

	if pin.ImageURL != "image.jpg" {
		t.Fatalf("unexpected image url: %q", pin.ImageURL)
	}

	if pin.Name != "Beach" {
		t.Fatalf("unexpected name: %q", pin.Name)
	}
}

func TestCreatePinValidation(t *testing.T) {
	service := NewService(&fakePinRepository{})

	tests := []model.CreatePinInput{
		{
			ImageURL: "",
			Name:     "Beach",
		},
		{
			ImageURL: "https://example.com/image.jpg",
			Name:     "",
		},
	}

	for _, input := range tests {
		_, err := service.Create(
			context.Background(),
			42,
			input,
		)

		if !errors.Is(err, ErrValidation) {
			t.Fatalf(
				"expected validation error, got %v",
				err,
			)
		}
	}
}

func TestListPins(t *testing.T) {
	now := time.Now()

	repo := &fakePinRepository{
		listResult: []model.Pin{
			{
				ID:        1,
				CreatedAt: now,
			},
			{
				ID:        2,
				CreatedAt: now.Add(-time.Minute),
			},
			{
				ID:        3,
				CreatedAt: now.Add(-2 * time.Minute),
			},
		},
		listHasNext: true,
	}

	service := NewService(repo)

	page, err := service.List(
		context.Background(),
		2,
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(page.Pins) != 2 {
		t.Fatalf("expected 2 pins, got %d", len(page.Pins))
	}

	if page.NextCursor == nil {
		t.Fatal("expected next cursor")
	}

	if page.NextCursor.ID != 2 {
		t.Fatalf(
			"expected cursor id 2, got %d",
			page.NextCursor.ID,
		)
	}
}

func TestListInvalidLimit(t *testing.T) {
	service := NewService(&fakePinRepository{})

	_, err := service.List(
		context.Background(),
		0,
		nil,
	)

	if !errors.Is(err, ErrValidation) {
		t.Fatalf(
			"expected validation error, got %v",
			err,
		)
	}
}

func TestListTooLargeLimit(t *testing.T) {
	service := NewService(&fakePinRepository{})

	_, err := service.List(
		context.Background(),
		101,
		nil,
	)

	if !errors.Is(err, ErrValidation) {
		t.Fatalf(
			"expected validation error, got %v",
			err,
		)
	}
}

func TestGetPin(t *testing.T) {
	repo := &fakePinRepository{
		getResult: model.Pin{
			ID:   10,
			Name: "Beach",
		},
	}

	service := NewService(repo)

	pin, err := service.GetByID(
		context.Background(),
		10,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pin.ID != 10 {
		t.Fatalf("expected id 10, got %d", pin.ID)
	}
}

func TestGetPinInvalidID(t *testing.T) {
	service := NewService(&fakePinRepository{})

	_, err := service.GetByID(
		context.Background(),
		0,
	)

	if !errors.Is(err, ErrValidation) {
		t.Fatalf(
			"expected validation error, got %v",
			err,
		)
	}
}

func TestGetPinNotFound(t *testing.T) {
	repo := &fakePinRepository{
		getErr: ErrPinNotFound,
	}

	service := NewService(repo)

	_, err := service.GetByID(
		context.Background(),
		10,
	)

	if !errors.Is(err, ErrPinNotFound) {
		t.Fatalf(
			"expected not found, got %v",
			err,
		)
	}
}

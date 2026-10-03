package pins

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"2026_2_PinPals/internal/model"
)

type Cursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        int       `json:"id"`
}

type Page struct {
	Pins       []model.Pin
	NextCursor *Cursor
}

func encodeCursor(cursor Cursor) (string, error) {
	data, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decodeCursor(value string) (*Cursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, errors.New("invalid cursor encoding")
	}

	var cursor Cursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return nil, errors.New("invalid cursor payload")
	}

	if cursor.ID < 1 || cursor.CreatedAt.IsZero() {
		return nil, errors.New("invalid cursor values")
	}

	return &cursor, nil
}

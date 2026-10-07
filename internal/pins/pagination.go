package pins

import (
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

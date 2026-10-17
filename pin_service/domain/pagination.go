package domain

import "time"

type Cursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        int       `json:"id"`
}

type Page struct {
	Pins       []Pin
	NextCursor *Cursor
}

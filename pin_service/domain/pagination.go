package domain

import "time"

type Cursor struct {
	CreatedAt time.Time
	ID        int
}

type Page struct {
	Pins       []Pin
	NextCursor *Cursor
}

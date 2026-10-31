package domain

import "time"

const (
	EntryStatusPublic      int16 = 0 // открытая
	EntryStatusSubscribers int16 = 1 // только для подписчиков
	EntryStatusPrivate     int16 = 2 // только для автора
)

type Desk struct {
	ID           int
	CreatorID    int
	Name         string
	Description  *string
	AvatarImgURL *string
	EntryStatus  int16
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}

type Pin struct {
	ID          int
	CreatorID   int
	ImageURL    string
	Name        string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type DeskWithPins struct {
	Desk
	Pins []Pin
}

type CreateDeskInput struct {
	Name         string
	Description  *string
	EntryStatus  int16
	AvatarImgURL *string
}

type UpdateDeskInput struct {
	Name            *string
	Description     *string
	DescriptionSet  bool
	EntryStatus     *int16
	AvatarImgURL    *string
	AvatarImgURLSet bool
}

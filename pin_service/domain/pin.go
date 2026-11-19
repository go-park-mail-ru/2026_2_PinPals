package domain

import (
	"io"
	"time"
)

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

type Author struct {
	ID        int
	Name      string
	UserTag   string
	AvatarURL *string
}

type PinWithAuthor struct {
	Pin
	Author Author
}

type CreatePinInput struct {
	ImageURL    string
	Name        string
	Description *string
	Tags        []string
}

type UpdatePinInput struct {
	Name        *string
	Description *string
}

type Tag struct {
	ID   int
	Name string
}

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

type DeskWithPins struct {
	Desk
	Pins []Pin
}

const DeskPreviewPinsLimit = 3

type CreateDeskInput struct {
	Name         string
	Description  *string
	EntryStatus  int16
	AvatarImgURL *string
	Avatar       io.Reader
}

type UpdateDeskInput struct {
	Name            *string
	Description     *string
	DescriptionSet  bool
	EntryStatus     *int16
	AvatarImgURL    *string
	AvatarImgURLSet bool
	Avatar          io.Reader
}

type LikeStats struct {
	Count     int  `json:"likes_count"`
	LikedByMe bool `json:"liked_by_me"`
}

package domain

import (
	"time"

	"github.com/google/uuid"
)

type Favorite struct {
	ID            uuid.UUID `db:"id" json:"id"`
	UserID        uuid.UUID `db:"user_account_id" json:"user_id"`
	DestinationID uuid.UUID `db:"destination_id" json:"destination_id"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

type FavoriteListItem struct {
	Favorite
	DestinationName string  `db:"destination_name" json:"destination_name"`
	DestinationSlug *string `db:"destination_slug" json:"destination_slug,omitempty"`
	City            *string `db:"city" json:"city,omitempty"`
	Country         *string `db:"country" json:"country,omitempty"`
	Category        *string `db:"category" json:"category,omitempty"`
	HeroImageURL    *string `db:"hero_image_url" json:"hero_image_url,omitempty"`
}

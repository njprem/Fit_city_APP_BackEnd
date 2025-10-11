package domain

import (
	"time"

	"github.com/google/uuid"
)

type Review struct {
	ID            uuid.UUID `db:"id" json:"id"`
	DestinationID uuid.UUID `db:"destination_id" json:"destination_id"`
	UserID        uuid.UUID `db:"user_id" json:"user_id"`
	Rating        int       `db:"rating" json:"rating"`
	Title         *string   `db:"title" json:"title,omitempty"`
	Content       *string   `db:"content" json:"content,omitempty"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`
}

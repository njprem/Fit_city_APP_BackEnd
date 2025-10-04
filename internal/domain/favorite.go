package domain

import (
	"time"

	"github.com/google/uuid"
)

type Favorite struct {
	ID            uuid.UUID  `db:"id" json:"id"`
	UserID        uuid.UUID  `db:"user_account_id" json:"user_id"`
	DestinationID *uuid.UUID `db:"destination_id" json:"destination_id,omitempty"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
}

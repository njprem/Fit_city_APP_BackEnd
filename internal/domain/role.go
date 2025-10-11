package domain

import (
	"time"

	"github.com/google/uuid"
)

type Role struct {
	ID          uuid.UUID `db:"id" json:"id"`
	Name        string    `db:"role_name" json:"role_name"`
	Description *string   `db:"description" json:"description,omitempty"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

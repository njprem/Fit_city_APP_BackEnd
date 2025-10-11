package domain

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID        int64     `db:"id" json:"id"`
	UserID    uuid.UUID `db:"user_id" json:"user_id"`
	Token     string    `db:"token" json:"token"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	ExpiresAt time.Time `db:"expires_at" json:"expires_at"`
	IsActive  bool      `db:"is_active" json:"is_active"`
}

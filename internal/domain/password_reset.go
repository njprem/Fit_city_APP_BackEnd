package domain

import (
	"time"

	"github.com/google/uuid"
)

type PasswordReset struct {
	ID        int64     `db:"id" json:"id"`
	UserID    uuid.UUID `db:"user_id" json:"user_id"`
	OTPHash   []byte    `db:"otp_hash" json:"-"`
	OTPSalt   []byte    `db:"otp_salt" json:"-"`
	ExpiresAt time.Time `db:"expires_at" json:"expires_at"`
	Consumed  bool      `db:"consumed" json:"consumed"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

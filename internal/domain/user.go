package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID               uuid.UUID `db:"id" json:"id"`
	Email            string    `db:"email" json:"email"`
	Username         *string   `db:"username" json:"username,omitempty"`
	FullName         *string   `db:"full_name" json:"full_name,omitempty"`
	ImageURL         *string   `db:"user_image_url" json:"user_image_url,omitempty"`
	PasswordHash     []byte    `db:"password_hash" json:"-"`
	PasswordSalt     []byte    `db:"password_salt" json:"-"`
	ProfileCompleted bool      `db:"profile_completed" json:"profile_completed"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at" json:"updated_at"`
	Roles            []Role    `db:"-" json:"roles,omitempty"`
}

func (u *User) HasRole(roleID uuid.UUID) bool {
	for _, role := range u.Roles {
		if role.ID == roleID {
			return true
		}
	}
	return false
}

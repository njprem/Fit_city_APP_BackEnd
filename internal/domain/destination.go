package domain

import (
	"time"

	"github.com/google/uuid"
)

type Destination struct {
	ID          uuid.UUID `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	City        *string   `db:"city" json:"city,omitempty"`
	Country     *string   `db:"country" json:"country,omitempty"`
	Category    *string   `db:"category" json:"category,omitempty"`
	Description *string   `db:"description" json:"description,omitempty"`
	Latitude    *float64  `db:"latitude" json:"latitude,omitempty"`
	Longitude   *float64  `db:"longitude" json:"longitude,omitempty"`
	HeroImage   *string   `db:"hero_image_url" json:"hero_image_url,omitempty"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}

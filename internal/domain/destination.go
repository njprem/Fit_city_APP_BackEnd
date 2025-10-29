package domain

import (
	"time"

	"github.com/google/uuid"
)

type DestinationStatus string

const (
	DestinationStatusDraft     DestinationStatus = "draft"
	DestinationStatusPublished DestinationStatus = "published"
	DestinationStatusArchived  DestinationStatus = "archived"
)

type Destination struct {
	ID          uuid.UUID         `db:"id" json:"id"`
	Name        string            `db:"name" json:"name"`
	Slug        *string           `db:"slug" json:"slug,omitempty"`
	Status      DestinationStatus `db:"status" json:"status"`
	Version     int64             `db:"version" json:"version"`
	City        *string           `db:"city" json:"city,omitempty"`
	Country     *string           `db:"country" json:"country,omitempty"`
	Category    *string           `db:"category" json:"category,omitempty"`
	Description *string           `db:"description" json:"description,omitempty"`
	Latitude    *float64          `db:"latitude" json:"latitude,omitempty"`
	Longitude   *float64          `db:"longitude" json:"longitude,omitempty"`
	HeroImage   *string           `db:"hero_image_url" json:"hero_image_url,omitempty"`
	CreatedAt   time.Time         `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time         `db:"updated_at" json:"updated_at"`
	UpdatedBy   *uuid.UUID        `db:"updated_by" json:"updated_by,omitempty"`
	DeletedAt   *time.Time        `db:"deleted_at" json:"deleted_at,omitempty"`
}

func (d Destination) IsPublished() bool {
	return d.Status == DestinationStatusPublished
}

func (d Destination) IsArchived() bool {
	return d.Status == DestinationStatusArchived
}

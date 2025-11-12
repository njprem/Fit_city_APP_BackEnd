package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type DestinationChangeAction string

const (
	DestinationChangeActionCreate DestinationChangeAction = "create"
	DestinationChangeActionUpdate DestinationChangeAction = "update"
	DestinationChangeActionDelete DestinationChangeAction = "delete"
)

type DestinationChangeStatus string

const (
	DestinationChangeStatusDraft         DestinationChangeStatus = "draft"
	DestinationChangeStatusPendingReview DestinationChangeStatus = "pending_review"
	DestinationChangeStatusApproved      DestinationChangeStatus = "approved"
	DestinationChangeStatusRejected      DestinationChangeStatus = "rejected"
)

type DestinationChangeFields struct {
	Name               *string             `json:"name,omitempty"`
	Slug               *string             `json:"slug,omitempty"`
	City               *string             `json:"city,omitempty"`
	Country            *string             `json:"country,omitempty"`
	Category           *string             `json:"category,omitempty"`
	Description        *string             `json:"description,omitempty"`
	Latitude           *float64            `json:"latitude,omitempty"`
	Longitude          *float64            `json:"longitude,omitempty"`
	Contact            *string             `json:"contact,omitempty"`
	OpeningTime        *string             `json:"opening_time,omitempty"`
	ClosingTime        *string             `json:"closing_time,omitempty"`
	Gallery            *DestinationGallery `json:"gallery,omitempty"`
	Status             *DestinationStatus  `json:"status,omitempty"`
	HeroImageUploadID  *string             `json:"hero_image_upload_id,omitempty"`
	HeroImageURL       *string             `json:"hero_image_url,omitempty"`
	PublishedHeroImage *string             `json:"published_hero_image,omitempty"`
	HardDelete         *bool               `json:"hard_delete,omitempty"`
}

func (f DestinationChangeFields) Value() (driver.Value, error) {
	data, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (f *DestinationChangeFields) Scan(value any) error {
	if value == nil {
		*f = DestinationChangeFields{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("destination change fields must be []byte")
	}
	return json.Unmarshal(bytes, f)
}

type DestinationChangeRequest struct {
	ID               uuid.UUID               `db:"id" json:"id"`
	DestinationID    *uuid.UUID              `db:"destination_id" json:"destination_id,omitempty"`
	Action           DestinationChangeAction `db:"action" json:"action"`
	Payload          DestinationChangeFields `db:"payload" json:"fields"`
	HeroImageTempKey *string                 `db:"hero_image_temp_key" json:"hero_image_temp_key,omitempty"`
	Status           DestinationChangeStatus `db:"status" json:"status"`
	DraftVersion     int                     `db:"draft_version" json:"draft_version"`
	SubmittedBy      uuid.UUID               `db:"submitted_by" json:"submitted_by"`
	ReviewedBy       *uuid.UUID              `db:"reviewed_by" json:"reviewed_by,omitempty"`
	SubmittedAt      *time.Time              `db:"submitted_at" json:"submitted_at,omitempty"`
	ReviewedAt       *time.Time              `db:"reviewed_at" json:"reviewed_at,omitempty"`
	ReviewMessage    *string                 `db:"review_message" json:"review_message,omitempty"`
	PublishedVersion *int64                  `db:"published_version" json:"published_version,omitempty"`
	CreatedAt        time.Time               `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time               `db:"updated_at" json:"updated_at"`
}

func (c DestinationChangeRequest) IsDraft() bool {
	return c.Status == DestinationChangeStatusDraft
}

func (c DestinationChangeRequest) IsPendingReview() bool {
	return c.Status == DestinationChangeStatusPendingReview
}

type DestinationChangeFilter struct {
	DestinationID *uuid.UUID
	SubmittedBy   *uuid.UUID
	Statuses      []DestinationChangeStatus
	Limit         int
	Offset        int
}

type DestinationSnapshot struct {
	ID          uuid.UUID          `json:"id"`
	Name        string             `json:"name"`
	Slug        *string            `json:"slug,omitempty"`
	Status      DestinationStatus  `json:"status"`
	Version     int64              `json:"version"`
	City        *string            `json:"city,omitempty"`
	Country     *string            `json:"country,omitempty"`
	Category    *string            `json:"category,omitempty"`
	Description *string            `json:"description,omitempty"`
	Latitude    *float64           `json:"latitude,omitempty"`
	Longitude   *float64           `json:"longitude,omitempty"`
	Contact     *string            `json:"contact,omitempty"`
	OpeningTime *string            `json:"opening_time,omitempty"`
	ClosingTime *string            `json:"closing_time,omitempty"`
	Gallery     DestinationGallery `json:"gallery,omitempty"`
	HeroImage   *string            `json:"hero_image_url,omitempty"`
	UpdatedAt   time.Time          `json:"updated_at"`
	UpdatedBy   *uuid.UUID         `json:"updated_by,omitempty"`
}

func (s DestinationSnapshot) Value() (driver.Value, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *DestinationSnapshot) Scan(value any) error {
	if value == nil {
		return errors.New("destination snapshot cannot be null")
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("expected []byte for destination snapshot, got %T", value)
	}
	return json.Unmarshal(bytes, s)
}

type DestinationVersion struct {
	ID              uuid.UUID           `db:"id" json:"id"`
	DestinationID   uuid.UUID           `db:"destination_id" json:"destination_id"`
	ChangeRequestID *uuid.UUID          `db:"change_request_id" json:"change_request_id,omitempty"`
	Version         int64               `db:"version" json:"version"`
	Snapshot        DestinationSnapshot `db:"snapshot" json:"snapshot"`
	CreatedAt       time.Time           `db:"created_at" json:"created_at"`
	CreatedBy       uuid.UUID           `db:"created_by" json:"created_by"`
}

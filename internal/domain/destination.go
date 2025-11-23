package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type DestinationStatus string

const (
	DestinationStatusDraft     DestinationStatus = "draft"
	DestinationStatusPublished DestinationStatus = "published"
	DestinationStatusArchived  DestinationStatus = "archived"
)

type DestinationMedia struct {
	URL      string  `json:"url"`
	Caption  *string `json:"caption,omitempty"`
	Ordering int     `json:"ordering,omitempty"`
}

type DestinationGallery []DestinationMedia

func (g DestinationGallery) Value() (driver.Value, error) {
	if len(g) == 0 {
		return nil, nil
	}
	data, err := json.Marshal(g)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (g *DestinationGallery) Scan(value any) error {
	if g == nil {
		return errors.New("destination gallery scan on nil receiver")
	}
	if value == nil {
		*g = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("destination gallery expected []byte, got %T", value)
	}
	var items []DestinationMedia
	if err := json.Unmarshal(bytes, &items); err != nil {
		return err
	}
	*g = DestinationGallery(items)
	return nil
}

type Destination struct {
	ID            uuid.UUID          `db:"id" json:"id"`
	Name          string             `db:"name" json:"name"`
	Slug          *string            `db:"slug" json:"slug,omitempty"`
	Status        DestinationStatus  `db:"status" json:"status"`
	Version       int64              `db:"version" json:"version"`
	City          *string            `db:"city" json:"city,omitempty"`
	Country       *string            `db:"country" json:"country,omitempty"`
	Category      *string            `db:"category" json:"category,omitempty"`
	Description   *string            `db:"description" json:"description,omitempty"`
	Latitude      *float64           `db:"latitude" json:"latitude,omitempty"`
	Longitude     *float64           `db:"longitude" json:"longitude,omitempty"`
	Contact       *string            `db:"contact" json:"contact,omitempty"`
	OpeningTime   *string            `db:"opening_time" json:"opening_time,omitempty"`
	ClosingTime   *string            `db:"closing_time" json:"closing_time,omitempty"`
	Gallery       DestinationGallery `db:"gallery" json:"gallery,omitempty"`
	HeroImage     *string            `db:"hero_image_url" json:"hero_image_url,omitempty"`
	CreatedAt     time.Time          `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time          `db:"updated_at" json:"updated_at"`
	UpdatedBy     *uuid.UUID         `db:"updated_by" json:"updated_by,omitempty"`
	DeletedAt     *time.Time         `db:"deleted_at" json:"deleted_at,omitempty"`
	AverageRating float64            `db:"average_rating" json:"-"`
	ReviewCount   int                `db:"review_count" json:"-"`
}

func (d Destination) IsPublished() bool {
	return d.Status == DestinationStatusPublished
}

func (d Destination) IsArchived() bool {
	return d.Status == DestinationStatusArchived
}

type DestinationListSort string

const (
	DestinationSortUpdatedAtDesc DestinationListSort = "updated_at_desc"
	DestinationSortRatingDesc    DestinationListSort = "rating_desc"
	DestinationSortRatingAsc     DestinationListSort = "rating_asc"
	DestinationSortNameAsc       DestinationListSort = "name_asc"
	DestinationSortNameDesc      DestinationListSort = "name_desc"
	DestinationSortSimilarity    DestinationListSort = "similarity"
	DestinationSortDistanceAsc   DestinationListSort = "distance"
)

func (s DestinationListSort) IsValid() bool {
	switch s {
	case DestinationSortUpdatedAtDesc,
		DestinationSortRatingDesc,
		DestinationSortRatingAsc,
		DestinationSortNameAsc,
		DestinationSortNameDesc,
		DestinationSortSimilarity,
		DestinationSortDistanceAsc:
		return true
	default:
		return false
	}
}

type DestinationListFilter struct {
	Search        string
	Categories    []string
	MinRating     *float64
	MaxRating     *float64
	City          *string
	Country       *string
	Latitude      *float64
	Longitude     *float64
	MaxDistanceKM *float64
	Sort          DestinationListSort
}

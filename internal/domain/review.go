package domain

import (
	"time"

	"github.com/google/uuid"
)

type Review struct {
	ID            uuid.UUID  `db:"id" json:"id"`
	DestinationID uuid.UUID  `db:"destination_id" json:"destination_id"`
	UserID        uuid.UUID  `db:"user_id" json:"user_id"`
	Rating        int        `db:"rating" json:"rating"`
	Title         *string    `db:"title" json:"title,omitempty"`
	Content       *string    `db:"content" json:"content,omitempty"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt     *time.Time `db:"deleted_at" json:"-"`
	DeletedBy     *uuid.UUID `db:"deleted_by" json:"-"`

	ReviewerName     *string `db:"reviewer_name" json:"-"`
	ReviewerUsername *string `db:"reviewer_username" json:"-"`
	ReviewerAvatar   *string `db:"reviewer_avatar_url" json:"-"`
	ReviewerEmail    *string `db:"reviewer_email" json:"-"`

	Media []ReviewMedia `json:"media,omitempty"`
}

type ReviewMedia struct {
	ID        uuid.UUID `db:"id" json:"id"`
	ReviewID  uuid.UUID `db:"review_id" json:"review_id"`
	ObjectKey string    `db:"object_key" json:"-"`
	URL       string    `db:"url" json:"url"`
	Ordering  int       `db:"ordering" json:"ordering"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type ReviewAggregate struct {
	DestinationID uuid.UUID   `json:"destination_id"`
	AverageRating float64     `json:"average_rating"`
	TotalReviews  int         `json:"total_reviews"`
	RatingCounts  map[int]int `json:"rating_counts"`
}

type ReviewListResult struct {
	DestinationID uuid.UUID       `json:"destination_id"`
	Reviews       []Review        `json:"reviews"`
	Aggregate     ReviewAggregate `json:"aggregate"`
	Limit         int             `json:"limit"`
	Offset        int             `json:"offset"`
}

type ReviewSortField string

const (
	ReviewSortCreatedAt ReviewSortField = "created_at"
	ReviewSortRating    ReviewSortField = "rating"
)

type SortOrder string

const (
	SortOrderAsc  SortOrder = "asc"
	SortOrderDesc SortOrder = "desc"
)

type ReviewListFilter struct {
	Limit        int
	Offset       int
	Rating       *int
	MinRating    *int
	MaxRating    *int
	PostedAfter  *time.Time
	PostedBefore *time.Time
	SortField    ReviewSortField
	SortOrder    SortOrder
}

type ReviewAggregateFilter struct {
	Rating       *int
	MinRating    *int
	MaxRating    *int
	PostedAfter  *time.Time
	PostedBefore *time.Time
}

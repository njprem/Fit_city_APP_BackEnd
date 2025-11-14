package domain

import (
	"time"

	"github.com/google/uuid"
)

type DestinationViewRange string

const (
	DestinationViewRange1h  DestinationViewRange = "1h"
	DestinationViewRange6h  DestinationViewRange = "6h"
	DestinationViewRange12h DestinationViewRange = "12h"
	DestinationViewRange24h DestinationViewRange = "24h"
	DestinationViewRange7d  DestinationViewRange = "7d"
	DestinationViewRange30d DestinationViewRange = "30d"
	DestinationViewRangeAll DestinationViewRange = "all"
)

var DestinationViewRangesOrdered = []DestinationViewRange{
	DestinationViewRange1h,
	DestinationViewRange6h,
	DestinationViewRange12h,
	DestinationViewRange24h,
	DestinationViewRange7d,
	DestinationViewRange30d,
	DestinationViewRangeAll,
}

func (r DestinationViewRange) Duration() (time.Duration, bool) {
	switch r {
	case DestinationViewRange1h:
		return time.Hour, true
	case DestinationViewRange6h:
		return 6 * time.Hour, true
	case DestinationViewRange12h:
		return 12 * time.Hour, true
	case DestinationViewRange24h:
		return 24 * time.Hour, true
	case DestinationViewRange7d:
		return 7 * 24 * time.Hour, true
	case DestinationViewRange30d:
		return 30 * 24 * time.Hour, true
	case DestinationViewRangeAll:
		return 0, true
	default:
		return 0, false
	}
}

type DestinationViewStatBucket struct {
	DestinationID uuid.UUID
	RangeKey      DestinationViewRange
	BucketStart   time.Time
	BucketEnd     time.Time
	TotalViews    int64
	UniqueUsers   int
	UniqueIPs     int
	UpdatedAt     time.Time
}

type DestinationViewStatValue struct {
	TotalViews  int64
	UniqueUsers int
	UniqueIPs   int
	BucketEnd   time.Time
}

type DestinationViewStats struct {
	DestinationID uuid.UUID
	Name          string
	City          *string
	Country       *string
	Ranges        map[DestinationViewRange]DestinationViewStatValue
}

type DestinationPopularityRecord struct {
	DestinationID uuid.UUID
	Name          string
	City          *string
	Country       *string
	Stats         map[DestinationViewRange]DestinationViewStatValue
}

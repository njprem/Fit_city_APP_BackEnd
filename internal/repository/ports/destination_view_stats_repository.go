package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

type DestinationViewStatsRepository interface {
	UpsertBuckets(ctx context.Context, buckets []domain.DestinationViewStatBucket) error
	GetStats(ctx context.Context, destinationID uuid.UUID) (map[domain.DestinationViewRange]domain.DestinationViewStatValue, time.Time, error)
	GetStatsBulk(ctx context.Context, destinationIDs []uuid.UUID) (map[uuid.UUID]map[domain.DestinationViewRange]domain.DestinationViewStatValue, map[uuid.UUID]time.Time, error)
	ListWithMetadata(ctx context.Context, destinationIDs []uuid.UUID) ([]domain.DestinationPopularityRecord, error)
	ListAllWithMetadata(ctx context.Context) ([]domain.DestinationPopularityRecord, error)
	ListTopByRange(ctx context.Context, rangeKey domain.DestinationViewRange, limit int) ([]domain.DestinationPopularityRecord, error)
	GetCheckpoint(ctx context.Context) (time.Time, error)
	UpdateCheckpoint(ctx context.Context, ts time.Time) error
	ListPublishedDestinationIDs(ctx context.Context) ([]uuid.UUID, error)
}

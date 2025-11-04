package ports

import (
	"context"

	"github.com/google/uuid"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

type ReviewRepository interface {
	Create(ctx context.Context, review *domain.Review) (*domain.Review, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Review, error)
	ListByDestination(ctx context.Context, destinationID uuid.UUID, filter domain.ReviewListFilter) ([]domain.Review, error)
	AggregateByDestination(ctx context.Context, destinationID uuid.UUID, filter domain.ReviewAggregateFilter) (*domain.ReviewAggregate, error)
	SoftDelete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}

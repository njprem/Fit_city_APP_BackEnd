package ports

import (
	"context"

	"github.com/google/uuid"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

type DestinationRepository interface {
	Create(ctx context.Context, fields domain.DestinationChangeFields, createdBy uuid.UUID, status domain.DestinationStatus, heroImageURL *string) (*domain.Destination, error)
	Update(ctx context.Context, id uuid.UUID, fields domain.DestinationChangeFields, updatedBy uuid.UUID, statusOverride *domain.DestinationStatus, heroImageURL *string) (*domain.Destination, error)
	Archive(ctx context.Context, id uuid.UUID, updatedBy uuid.UUID) (*domain.Destination, error)
	HardDelete(ctx context.Context, id uuid.UUID) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Destination, error)
	FindPublishedByID(ctx context.Context, id uuid.UUID) (*domain.Destination, error)
	FindBySlug(ctx context.Context, slug string) (*domain.Destination, error)
	ListPublished(ctx context.Context, limit, offset int, query string) ([]domain.Destination, error)
}

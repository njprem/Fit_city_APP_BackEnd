package ports

import (
	"context"

	"github.com/google/uuid"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

type DestinationVersionRepository interface {
	Create(ctx context.Context, version *domain.DestinationVersion) (*domain.DestinationVersion, error)
	ListByDestination(ctx context.Context, destinationID uuid.UUID, limit int) ([]domain.DestinationVersion, error)
}

package ports

import (
	"context"

	"github.com/google/uuid"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

type DestinationRepository interface {
	Create(ctx context.Context, destination *domain.Destination) (*domain.Destination, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Destination, error)
	List(ctx context.Context, limit, offset int) ([]domain.Destination, error)
	SearchByCity(ctx context.Context, city string, limit, offset int) ([]domain.Destination, error)
}

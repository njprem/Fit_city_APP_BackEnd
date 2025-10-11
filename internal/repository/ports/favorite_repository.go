package ports

import (
	"context"

	"github.com/google/uuid"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

type FavoriteRepository interface {
	Add(ctx context.Context, userID, destinationID uuid.UUID) (*domain.Favorite, error)
	Remove(ctx context.Context, userID, destinationID uuid.UUID) error
	ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Favorite, error)
}

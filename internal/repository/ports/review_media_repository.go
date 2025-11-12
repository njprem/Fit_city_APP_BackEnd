package ports

import (
	"context"

	"github.com/google/uuid"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

type ReviewMediaRepository interface {
	CreateMany(ctx context.Context, media []domain.ReviewMedia) error
	ListByReviewIDs(ctx context.Context, reviewIDs []uuid.UUID) (map[uuid.UUID][]domain.ReviewMedia, error)
}

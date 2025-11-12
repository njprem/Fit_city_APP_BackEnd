package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

type DestinationChangeRepository interface {
	Create(ctx context.Context, change *domain.DestinationChangeRequest) (*domain.DestinationChangeRequest, error)
	Update(ctx context.Context, change *domain.DestinationChangeRequest) (*domain.DestinationChangeRequest, error)
	MarkSubmitted(ctx context.Context, id uuid.UUID, submittedAt time.Time) (*domain.DestinationChangeRequest, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.DestinationChangeRequest, error)
	List(ctx context.Context, filter domain.DestinationChangeFilter) ([]domain.DestinationChangeRequest, error)
	SetStatus(ctx context.Context, id uuid.UUID, status domain.DestinationChangeStatus, reviewerID *uuid.UUID, reviewMessage *string, publishedVersion *int64) (*domain.DestinationChangeRequest, error)
	DeleteByID(ctx context.Context, id uuid.UUID) error
}

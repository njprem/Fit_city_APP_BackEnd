package ports

import (
	"context"

	"github.com/google/uuid"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

type DestinationImportRepository interface {
	CreateJob(ctx context.Context, job *domain.DestinationImportJob) (*domain.DestinationImportJob, error)
	UpdateJob(ctx context.Context, job *domain.DestinationImportJob) (*domain.DestinationImportJob, error)
	FindJobByID(ctx context.Context, id uuid.UUID) (*domain.DestinationImportJob, error)
	InsertRow(ctx context.Context, row *domain.DestinationImportRow) (*domain.DestinationImportRow, error)
	ListRowsByJob(ctx context.Context, jobID uuid.UUID) ([]domain.DestinationImportRow, error)
}

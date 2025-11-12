package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/repository/ports"
)

type DestinationVersionRepository struct {
	db *sqlx.DB
}

func NewDestinationVersionRepo(db *sqlx.DB) *DestinationVersionRepository {
	return &DestinationVersionRepository{db: db}
}

func (r *DestinationVersionRepository) Create(ctx context.Context, version *domain.DestinationVersion) (*domain.DestinationVersion, error) {
	const query = `
		INSERT INTO destination_version (destination_id, change_request_id, version, snapshot, created_at, created_by)
		VALUES (:destination_id, :change_request_id, :version, :snapshot, NOW(), :created_by)
		RETURNING id, destination_id, change_request_id, version, snapshot, created_at, created_by
	`

	args := map[string]any{
		"destination_id":    version.DestinationID,
		"change_request_id": nullableUUID(version.ChangeRequestID),
		"version":           version.Version,
		"snapshot":          version.Snapshot,
		"created_by":        version.CreatedBy,
	}

	rows, err := r.db.NamedQueryContext(ctx, query, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		var inserted domain.DestinationVersion
		if err := rows.StructScan(&inserted); err != nil {
			return nil, err
		}
		return &inserted, nil
	}
	return nil, nil
}

func (r *DestinationVersionRepository) ListByDestination(ctx context.Context, destinationID uuid.UUID, limit int) ([]domain.DestinationVersion, error) {
	const query = `
		SELECT id, destination_id, change_request_id, version, snapshot, created_at, created_by
		FROM destination_version
		WHERE destination_id = $1
		ORDER BY version DESC
		LIMIT $2
	`
	versions := make([]domain.DestinationVersion, 0)
	if err := r.db.SelectContext(ctx, &versions, query, destinationID, limit); err != nil {
		return nil, err
	}
	return versions, nil
}

var _ ports.DestinationVersionRepository = (*DestinationVersionRepository)(nil)

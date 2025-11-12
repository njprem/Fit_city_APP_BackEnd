package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/repository/ports"
)

type ReviewMediaRepository struct {
	db *sqlx.DB
}

func NewReviewMediaRepo(db *sqlx.DB) *ReviewMediaRepository {
	return &ReviewMediaRepository{db: db}
}

func (r *ReviewMediaRepository) CreateMany(ctx context.Context, media []domain.ReviewMedia) error {
	if len(media) == 0 {
		return nil
	}
	const query = `
		INSERT INTO review_media (review_id, object_key, url, ordering)
		VALUES (:review_id, :object_key, :url, :ordering)
	`

	for _, item := range media {
		args := map[string]any{
			"review_id":  item.ReviewID,
			"object_key": item.ObjectKey,
			"url":        item.URL,
			"ordering":   item.Ordering,
		}
		if _, err := r.db.NamedExecContext(ctx, query, args); err != nil {
			return err
		}
	}
	return nil
}

func (r *ReviewMediaRepository) ListByReviewIDs(ctx context.Context, reviewIDs []uuid.UUID) (map[uuid.UUID][]domain.ReviewMedia, error) {
	result := make(map[uuid.UUID][]domain.ReviewMedia, len(reviewIDs))
	if len(reviewIDs) == 0 {
		return result, nil
	}

	query, args, err := sqlx.In(`
		SELECT id, review_id, object_key, url, ordering, created_at
		FROM review_media
		WHERE review_id IN (?)
		ORDER BY review_id, ordering, created_at, id
	`, reviewIDs)
	if err != nil {
		return nil, err
	}
	query = r.db.Rebind(query)

	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var media domain.ReviewMedia
		if err := rows.StructScan(&media); err != nil {
			return nil, err
		}
		result[media.ReviewID] = append(result[media.ReviewID], media)
	}
	return result, rows.Err()
}

var _ ports.ReviewMediaRepository = (*ReviewMediaRepository)(nil)

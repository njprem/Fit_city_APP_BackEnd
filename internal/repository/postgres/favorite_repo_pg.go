package postgres

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/repository/ports"
)

type FavoriteRepository struct {
	db *sqlx.DB
}

func NewFavoriteRepo(db *sqlx.DB) *FavoriteRepository {
	return &FavoriteRepository{db: db}
}

func (r *FavoriteRepository) Add(ctx context.Context, userID, destinationID uuid.UUID) (*domain.Favorite, error) {
	const query = `
		INSERT INTO favorite_list (user_account_id, destination_id)
		VALUES ($1, $2)
		ON CONFLICT (user_account_id, destination_id) DO NOTHING
		RETURNING id, user_account_id, destination_id, created_at
	`

	var favorite domain.Favorite
	if err := r.db.GetContext(ctx, &favorite, query, userID, destinationID); err != nil {
		return nil, err
	}
	return &favorite, nil
}

func (r *FavoriteRepository) Remove(ctx context.Context, userID, destinationID uuid.UUID) error {
	const query = `
		DELETE FROM favorite_list
		WHERE user_account_id = $1 AND destination_id = $2
	`
	result, err := r.db.ExecContext(ctx, query, userID, destinationID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *FavoriteRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.FavoriteListItem, error) {
	const query = `
		SELECT
			f.id,
			f.user_account_id,
			f.destination_id,
			f.created_at,
			d.name AS destination_name,
			d.slug AS destination_slug,
			d.city,
			d.country,
			d.category,
			d.hero_image_url
		FROM favorite_list f
		JOIN travel_destination d ON d.id = f.destination_id
		WHERE f.user_account_id = $1
		  AND d.deleted_at IS NULL
		  AND d.status = 'published'
		ORDER BY f.created_at DESC, f.id DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryxContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.FavoriteListItem, 0)
	for rows.Next() {
		var item domain.FavoriteListItem
		if err := rows.StructScan(&item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *FavoriteRepository) CountByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	const query = `
		SELECT COUNT(*)
		FROM favorite_list
		WHERE user_account_id = $1
	`
	var count int64
	if err := r.db.GetContext(ctx, &count, query, userID); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *FavoriteRepository) CountByDestination(ctx context.Context, destinationID uuid.UUID) (int64, error) {
	const query = `
		SELECT COUNT(*)
		FROM favorite_list
		WHERE destination_id = $1
	`
	var count int64
	if err := r.db.GetContext(ctx, &count, query, destinationID); err != nil {
		return 0, err
	}
	return count, nil
}

var _ ports.FavoriteRepository = (*FavoriteRepository)(nil)

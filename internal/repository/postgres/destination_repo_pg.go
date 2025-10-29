package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/repository/ports"
)

type DestinationRepository struct {
	db *sqlx.DB
}

func NewDestinationRepo(db *sqlx.DB) *DestinationRepository {
	return &DestinationRepository{db: db}
}

func (r *DestinationRepository) Create(ctx context.Context, fields domain.DestinationChangeFields, createdBy uuid.UUID, status domain.DestinationStatus, heroImageURL *string) (*domain.Destination, error) {
	const query = `
		INSERT INTO travel_destination (
			name, slug, city, country, category, description,
			latitude, longitude, hero_image_url, status, version, updated_by
		) VALUES (
			:name, :slug, :city, :country, :category, :description,
			:latitude, :longitude, :hero_image_url, :status, 1, :updated_by
		)
		RETURNING id, name, slug, status, version, city, country, category, description,
		          latitude, longitude, hero_image_url, created_at, updated_at, updated_by, deleted_at
	`

	args := map[string]any{
		"name":           valueOrDefault(fields.Name, ""),
		"slug":           nullString(fields.Slug),
		"city":           nullString(fields.City),
		"country":        nullString(fields.Country),
		"category":       nullString(fields.Category),
		"description":    nullString(fields.Description),
		"latitude":       nullFloat(fields.Latitude),
		"longitude":      nullFloat(fields.Longitude),
		"hero_image_url": nullStringOr(heroImageURL),
		"status":         status,
		"updated_by":     createdBy,
	}

	rows, err := r.db.NamedQueryContext(ctx, query, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		var dest domain.Destination
		if err = rows.StructScan(&dest); err != nil {
			return nil, err
		}
		return &dest, nil
	}
	return nil, sql.ErrNoRows
}

func (r *DestinationRepository) Update(ctx context.Context, id uuid.UUID, fields domain.DestinationChangeFields, updatedBy uuid.UUID, statusOverride *domain.DestinationStatus, heroImageURL *string) (*domain.Destination, error) {
	setParts := []string{"updated_at = NOW()", "updated_by = $1", "version = version + 1"}
	args := []any{updatedBy}
	idx := 2

	if fields.Name != nil {
		setParts = append(setParts, fmt.Sprintf("name = $%d", idx))
		args = append(args, *fields.Name)
		idx++
	}
	if fields.Slug != nil {
		setParts = append(setParts, fmt.Sprintf("slug = $%d", idx))
		args = append(args, nullString(fields.Slug))
		idx++
	}
	if fields.City != nil {
		setParts = append(setParts, fmt.Sprintf("city = $%d", idx))
		args = append(args, nullString(fields.City))
		idx++
	}
	if fields.Country != nil {
		setParts = append(setParts, fmt.Sprintf("country = $%d", idx))
		args = append(args, nullString(fields.Country))
		idx++
	}
	if fields.Category != nil {
		setParts = append(setParts, fmt.Sprintf("category = $%d", idx))
		args = append(args, nullString(fields.Category))
		idx++
	}
	if fields.Description != nil {
		setParts = append(setParts, fmt.Sprintf("description = $%d", idx))
		args = append(args, nullString(fields.Description))
		idx++
	}
	if fields.Latitude != nil {
		setParts = append(setParts, fmt.Sprintf("latitude = $%d", idx))
		args = append(args, nullFloat(fields.Latitude))
		idx++
	}
	if fields.Longitude != nil {
		setParts = append(setParts, fmt.Sprintf("longitude = $%d", idx))
		args = append(args, nullFloat(fields.Longitude))
		idx++
	}

	if heroImageURL != nil {
		setParts = append(setParts, fmt.Sprintf("hero_image_url = $%d", idx))
		args = append(args, nullStringOr(heroImageURL))
		idx++
	}

	if statusOverride != nil {
		setParts = append(setParts, fmt.Sprintf("status = $%d", idx))
		args = append(args, *statusOverride)
		idx++
	}

	query := fmt.Sprintf(`
		UPDATE travel_destination
		SET %s
		WHERE id = $%d
		RETURNING id, name, slug, status, version, city, country, category, description,
		          latitude, longitude, hero_image_url, created_at, updated_at, updated_by, deleted_at
	`, strings.Join(setParts, ", "), idx)

	args = append(args, id)

	var dest domain.Destination
	if err := r.db.GetContext(ctx, &dest, query, args...); err != nil {
		return nil, err
	}
	return &dest, nil
}

func (r *DestinationRepository) Archive(ctx context.Context, id uuid.UUID, updatedBy uuid.UUID) (*domain.Destination, error) {
	const query = `
		UPDATE travel_destination
		SET status = 'archived',
		    deleted_at = NOW(),
		    updated_at = NOW(),
		    updated_by = $2,
		    version = version + 1
		WHERE id = $1
		RETURNING id, name, slug, status, version, city, country, category, description,
		          latitude, longitude, hero_image_url, created_at, updated_at, updated_by, deleted_at
	`
	var dest domain.Destination
	if err := r.db.GetContext(ctx, &dest, query, id, updatedBy); err != nil {
		return nil, err
	}
	return &dest, nil
}

func (r *DestinationRepository) HardDelete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM travel_destination WHERE id = $1`, id)
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

func (r *DestinationRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Destination, error) {
	const query = `
		SELECT id, name, slug, status, version, city, country, category, description,
		       latitude, longitude, hero_image_url, created_at, updated_at, updated_by, deleted_at
		FROM travel_destination
		WHERE id = $1
	`
	var dest domain.Destination
	if err := r.db.GetContext(ctx, &dest, query, id); err != nil {
		return nil, err
	}
	return &dest, nil
}

func (r *DestinationRepository) FindPublishedByID(ctx context.Context, id uuid.UUID) (*domain.Destination, error) {
	const query = `
		SELECT id, name, slug, status, version, city, country, category, description,
		       latitude, longitude, hero_image_url, created_at, updated_at, updated_by, deleted_at
		FROM travel_destination
		WHERE id = $1 AND status = 'published' AND deleted_at IS NULL
	`
	var dest domain.Destination
	if err := r.db.GetContext(ctx, &dest, query, id); err != nil {
		return nil, err
	}
	return &dest, nil
}

func (r *DestinationRepository) FindBySlug(ctx context.Context, slug string) (*domain.Destination, error) {
	const query = `
		SELECT id, name, slug, status, version, city, country, category, description,
		       latitude, longitude, hero_image_url, created_at, updated_at, updated_by, deleted_at
		FROM travel_destination
		WHERE slug = $1 AND deleted_at IS NULL
	`
	var dest domain.Destination
	if err := r.db.GetContext(ctx, &dest, query, slug); err != nil {
		return nil, err
	}
	return &dest, nil
}

func (r *DestinationRepository) ListPublished(ctx context.Context, limit, offset int) ([]domain.Destination, error) {
	const query = `
		SELECT id, name, slug, status, version, city, country, category, description,
		       latitude, longitude, hero_image_url, created_at, updated_at, updated_by, deleted_at
		FROM travel_destination
		WHERE status = 'published' AND deleted_at IS NULL
		ORDER BY updated_at DESC
		LIMIT $1 OFFSET $2
	`
	destinations := make([]domain.Destination, 0)
	if err := r.db.SelectContext(ctx, &destinations, query, limit, offset); err != nil {
		return nil, err
	}
	return destinations, nil
}

func valueOrDefault(ptr *string, fallback string) string {
	if ptr == nil || strings.TrimSpace(*ptr) == "" {
		return fallback
	}
	return strings.TrimSpace(*ptr)
}

func nullString(ptr *string) sql.NullString {
	if ptr == nil {
		return sql.NullString{Valid: false}
	}
	v := strings.TrimSpace(*ptr)
	if v == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: v, Valid: true}
}

func nullStringOr(ptr *string) sql.NullString {
	if ptr == nil {
		return sql.NullString{Valid: false}
	}
	val := strings.TrimSpace(*ptr)
	if val == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: val, Valid: true}
}

func nullFloat(ptr *float64) sql.NullFloat64 {
	if ptr == nil {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: *ptr, Valid: true}
}

var _ ports.DestinationRepository = (*DestinationRepository)(nil)

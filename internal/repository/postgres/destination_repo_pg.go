package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

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
			latitude, longitude, contact, opening_time, closing_time,
			gallery, hero_image_url, status, version, updated_by
		) VALUES (
			:name, :slug, :city, :country, :category, :description,
			:latitude, :longitude, :contact, :opening_time, :closing_time,
			:gallery, :hero_image_url, :status, 1, :updated_by
		)
		RETURNING id, name, slug, status, version, city, country, category, description,
		          latitude, longitude, contact, opening_time, closing_time, gallery,
		          hero_image_url, created_at, updated_at, updated_by, deleted_at
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
		"contact":        nullString(fields.Contact),
		"opening_time":   nullString(fields.OpeningTime),
		"closing_time":   nullString(fields.ClosingTime),
		"gallery":        galleryValue(fields.Gallery),
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
	if fields.Contact != nil {
		setParts = append(setParts, fmt.Sprintf("contact = $%d", idx))
		args = append(args, nullString(fields.Contact))
		idx++
	}
	if fields.OpeningTime != nil {
		setParts = append(setParts, fmt.Sprintf("opening_time = $%d", idx))
		args = append(args, nullString(fields.OpeningTime))
		idx++
	}
	if fields.ClosingTime != nil {
		setParts = append(setParts, fmt.Sprintf("closing_time = $%d", idx))
		args = append(args, nullString(fields.ClosingTime))
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
	if fields.Gallery != nil {
		setParts = append(setParts, fmt.Sprintf("gallery = $%d", idx))
		args = append(args, galleryValue(fields.Gallery))
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
		          latitude, longitude, contact, opening_time, closing_time, gallery,
		          hero_image_url, created_at, updated_at, updated_by, deleted_at
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
		          latitude, longitude, contact, opening_time, closing_time, gallery,
		          hero_image_url, created_at, updated_at, updated_by, deleted_at
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
		       latitude, longitude, contact, opening_time, closing_time, gallery,
		       hero_image_url, created_at, updated_at, updated_by, deleted_at
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
		       latitude, longitude, contact, opening_time, closing_time, gallery,
		       hero_image_url, created_at, updated_at, updated_by, deleted_at
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
		       latitude, longitude, contact, opening_time, closing_time, gallery,
		       hero_image_url, created_at, updated_at, updated_by, deleted_at
		FROM travel_destination
		WHERE slug = $1 AND deleted_at IS NULL
	`
	var dest domain.Destination
	if err := r.db.GetContext(ctx, &dest, query, slug); err != nil {
		return nil, err
	}
	return &dest, nil
}

func (r *DestinationRepository) ListPublished(ctx context.Context, limit, offset int, filter domain.DestinationListFilter) ([]domain.Destination, error) {
	const base = `
		SELECT
			d.id,
			d.name,
			d.slug,
			d.status,
			d.version,
			d.city,
			d.country,
			d.category,
			d.description,
			d.latitude,
			d.longitude,
			d.contact,
			d.opening_time,
			d.closing_time,
			d.gallery,
			d.hero_image_url,
			d.created_at,
			d.updated_at,
			d.updated_by,
			d.deleted_at,
			COALESCE(AVG(r.rating)::float8, 0) AS average_rating,
			COUNT(r.id)::int AS review_count
		FROM travel_destination d
		LEFT JOIN review r ON r.destination_id = d.id AND r.deleted_at IS NULL
		WHERE d.status = 'published' AND d.deleted_at IS NULL
	`

	params := make([]any, 0, 6)
	var builder strings.Builder
	builder.WriteString(base)

	if trimmed := strings.TrimSpace(filter.Search); trimmed != "" {
		placeholder := fmt.Sprintf("$%d", len(params)+1)
		builder.WriteString(`
		AND to_tsvector(
			'simple',
			COALESCE(d.name, '') || ' ' ||
			COALESCE(d.city, '') || ' ' ||
			COALESCE(d.country, '') || ' ' ||
			COALESCE(d.category, '') || ' ' ||
			COALESCE(d.description, '')
		) @@ plainto_tsquery('simple', ` + placeholder + `)
		 OR similarity(d.name, ` + placeholder + `) > 0.2
		`)
		params = append(params, trimmed)
	}

	if len(filter.Categories) > 0 {
		categories := make([]string, 0, len(filter.Categories))
		for _, category := range filter.Categories {
			if trimmed := strings.TrimSpace(category); trimmed != "" {
				categories = append(categories, trimmed)
			}
		}
		if len(categories) > 0 {
			placeholder := fmt.Sprintf("$%d", len(params)+1)
			builder.WriteString(`
		AND d.category = ANY(` + placeholder + `)
			`)
			params = append(params, pq.StringArray(categories))
		}
	}

	if filter.City != nil {
		city := strings.TrimSpace(*filter.City)
		if city != "" {
			placeholder := fmt.Sprintf("$%d", len(params)+1)
			builder.WriteString("\n\tAND d.city ILIKE " + placeholder)
			params = append(params, "%"+city+"%")
		}
	}

	if filter.Country != nil {
		country := strings.TrimSpace(*filter.Country)
		if country != "" {
			placeholder := fmt.Sprintf("$%d", len(params)+1)
			builder.WriteString("\n\tAND d.country ILIKE " + placeholder)
			params = append(params, "%"+country+"%")
		}
	}

	if filter.MaxDistanceKM != nil && filter.Latitude != nil && filter.Longitude != nil {
		latPlaceholder := fmt.Sprintf("$%d", len(params)+1)
		lngPlaceholder := fmt.Sprintf("$%d", len(params)+2)
		distPlaceholder := fmt.Sprintf("$%d", len(params)+3)

		builder.WriteString(`
			AND d.latitude IS NOT NULL AND d.longitude IS NOT NULL
			AND (6371 * acos(
				cos(radians(` + latPlaceholder + `)) * cos(radians(d.latitude)) *
				cos(radians(d.longitude) - radians(` + lngPlaceholder + `)) +
				sin(radians(` + latPlaceholder + `)) * sin(radians(d.latitude)) 
			)) <= ` + distPlaceholder + `
		`)
		params = append(params, *filter.Latitude, *filter.Longitude, *filter.MaxDistanceKM)
	}

	builder.WriteString(`
		GROUP BY d.id
	`)

	havingClauses := make([]string, 0, 2)
	if filter.MinRating != nil {
		placeholder := fmt.Sprintf("$%d", len(params)+1)
		havingClauses = append(havingClauses, "COALESCE(AVG(r.rating)::float8, 0) >= "+placeholder)
		params = append(params, *filter.MinRating)
	}
	if filter.MaxRating != nil {
		placeholder := fmt.Sprintf("$%d", len(params)+1)
		havingClauses = append(havingClauses, "COALESCE(AVG(r.rating)::float8, 0) <= "+placeholder)
		params = append(params, *filter.MaxRating)
	}
	if len(havingClauses) > 0 {
		builder.WriteString("\n\tHAVING " + strings.Join(havingClauses, " AND "))
	}

	builder.WriteString("\n\tORDER BY ")
	switch filter.Sort {
	case domain.DestinationSortRatingAsc:
		builder.WriteString("average_rating ASC, d.name ASC")
	case domain.DestinationSortRatingDesc:
		builder.WriteString("average_rating DESC, d.name ASC")
	case domain.DestinationSortNameAsc:
		builder.WriteString("d.name ASC")
	case domain.DestinationSortNameDesc:
		builder.WriteString("d.name DESC")
	case domain.DestinationSortSimilarity:
		trimmed := strings.TrimSpace(filter.Search)
		if trimmed != "" {
			placeholder := fmt.Sprintf("$%d", len(params)+1)
			builder.WriteString("similarity(d.name, " + placeholder + ") DESC, d.name ASC")
			params = append(params, trimmed)
		}
	case domain.DestinationSortDistanceAsc:
		if filter.Latitude != nil && filter.Longitude != nil {
			latPlaceholder := fmt.Sprintf("$%d", len(params)+1)
			lngPlaceholder := fmt.Sprintf("$%d", len(params)+2)

			builder.WriteString(`
				(6371 * acos(
					cos(radians(` + latPlaceholder + `)) * cos(radians(d.latitude)) *
					cos(radians(d.longitude) - radians(` + lngPlaceholder + `)) + 
					sin(radians(` + latPlaceholder + `)) * sin(radians(d.latitude))
				)) ASc, d.name ASC
			`)
		}
	default:
		builder.WriteString("d.updated_at DESC")
	}

	limitPlaceholder := fmt.Sprintf("$%d", len(params)+1)
	offsetPlaceholder := fmt.Sprintf("$%d", len(params)+2)
	builder.WriteString(`
		LIMIT ` + limitPlaceholder + ` OFFSET ` + offsetPlaceholder + `
	`)
	params = append(params, limit, offset)

	destinations := make([]domain.Destination, 0)
	if err := r.db.SelectContext(ctx, &destinations, builder.String(), params...); err != nil {
		return nil, err
	}
	return destinations, nil
}

func (r *DestinationRepository) Autocomplete(ctx context.Context, query string, limit int) ([]string, error) {
	q := strings.TrimSpace(query)
	args := []any{q, limit}

	sql := `
		SELECT DISTINCT suggestion
		FROM (
			SELECT
				name AS suggestion,
				similarity(name, $1) AS score
			FROM travel_destination
			WHERE status = 'published' AND name IS NOT NULL

			UNION
			SELECT
				city AS suggestion,
				similarity(city, $1) AS score
			FROM travel_destination
			WHERE status = 'published' AND city IS NOT NULL

			UNION
			SELECT
				country AS suggestion,
				similarity(country, $1) AS score
			FROM travel_destination
			WHERE status = 'published' AND country IS NOT NULL
		) AS s
		WHERE suggestionn ILIKE '%' || $1 || '%'
		ORDER BY score DESC
		LIMIT $2;
	`

	var results []string
	if err := r.db.SelectContext(ctx, &results, sql, args...); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		sql = `
			SELECT name
			FROM travel_destination
			WHERE status = 'published'
				AND name ILIKE '%' || $1 || '$'
			ORDER BY name ASC
			LIMIT $2
		`

		if err := r.db.SelectContext(ctx, &results, sql, args...); err != nil {
			return nil, err
		}
	}

	return results, nil
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

func galleryValue(ptr *domain.DestinationGallery) domain.DestinationGallery {
	if ptr == nil {
		return nil
	}
	return append(domain.DestinationGallery(nil), (*ptr)...)
}

var _ ports.DestinationRepository = (*DestinationRepository)(nil)

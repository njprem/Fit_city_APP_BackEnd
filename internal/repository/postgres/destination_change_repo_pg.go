package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/repository/ports"
)

type DestinationChangeRepository struct {
	db *sqlx.DB
}

func NewDestinationChangeRepo(db *sqlx.DB) *DestinationChangeRepository {
	return &DestinationChangeRepository{db: db}
}

func (r *DestinationChangeRepository) Create(ctx context.Context, change *domain.DestinationChangeRequest) (*domain.DestinationChangeRequest, error) {
	const query = `
		INSERT INTO destination_change_request (
			destination_id, action, payload, hero_image_temp_key, status,
			draft_version, submitted_by, created_at, updated_at
		) VALUES (
			:destination_id, :action, :payload, :hero_image_temp_key, :status,
			:draft_version, :submitted_by, NOW(), NOW()
		)
		RETURNING id, destination_id, action, payload, hero_image_temp_key, status,
		          draft_version, submitted_by, reviewed_by, submitted_at, reviewed_at,
		          review_message, published_version, created_at, updated_at
	`

	args := map[string]any{
		"destination_id":      nullableUUID(change.DestinationID),
		"action":              change.Action,
		"payload":             change.Payload,
		"hero_image_temp_key": nullString(change.HeroImageTempKey),
		"status":              change.Status,
		"draft_version":       change.DraftVersion,
		"submitted_by":        change.SubmittedBy,
	}

	rows, err := r.db.NamedQueryContext(ctx, query, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		var inserted domain.DestinationChangeRequest
		if err := rows.StructScan(&inserted); err != nil {
			return nil, err
		}
		return &inserted, nil
	}
	return nil, sql.ErrNoRows
}

func (r *DestinationChangeRepository) Update(ctx context.Context, change *domain.DestinationChangeRequest) (*domain.DestinationChangeRequest, error) {
	prevVersion := change.DraftVersion
	if prevVersion > 0 {
		prevVersion--
	}

	const queryTemplate = `
		UPDATE destination_change_request
		SET payload = $2,
		    hero_image_temp_key = $3,
		    draft_version = $4,
		    status = $5,
		    submitted_at = $6,
		    reviewed_at = $7,
		    reviewed_by = $8,
		    review_message = $9,
		    published_version = $10,
		    updated_at = NOW()
		WHERE id = $1 %s
		RETURNING id, destination_id, action, payload, hero_image_temp_key, status,
		          draft_version, submitted_by, reviewed_by, submitted_at, reviewed_at,
		          review_message, published_version, created_at, updated_at
	`

	where := ""
	args := []any{
		change.ID,
		change.Payload,
		nullString(change.HeroImageTempKey),
		change.DraftVersion,
		change.Status,
		nullTime(change.SubmittedAt),
		nullTime(change.ReviewedAt),
		nullableUUID(change.ReviewedBy),
		nullString(change.ReviewMessage),
		change.PublishedVersion,
	}

	if prevVersion > 0 {
		where = "AND draft_version = $11"
		args = append(args, prevVersion)
	}

	query := fmt.Sprintf(queryTemplate, where)

	var updated domain.DestinationChangeRequest
	if err := r.db.GetContext(ctx, &updated, query, args...); err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *DestinationChangeRepository) MarkSubmitted(ctx context.Context, id uuid.UUID, submittedAt time.Time) (*domain.DestinationChangeRequest, error) {
	const query = `
		UPDATE destination_change_request
		SET status = 'pending_review',
		    submitted_at = $2,
		    updated_at = NOW()
		WHERE id = $1 AND status IN ('draft', 'rejected')
		RETURNING id, destination_id, action, payload, hero_image_temp_key, status,
		          draft_version, submitted_by, reviewed_by, submitted_at, reviewed_at,
		          review_message, published_version, created_at, updated_at
	`
	var updated domain.DestinationChangeRequest
	if err := r.db.GetContext(ctx, &updated, query, id, submittedAt); err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *DestinationChangeRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.DestinationChangeRequest, error) {
	const query = `
		SELECT id, destination_id, action, payload, hero_image_temp_key, status,
		       draft_version, submitted_by, reviewed_by, submitted_at, reviewed_at,
		       review_message, published_version, created_at, updated_at,
		       COUNT(*) OVER() AS total_count
		FROM destination_change_request
		WHERE id = $1
	`
	var change domain.DestinationChangeRequest
	if err := r.db.GetContext(ctx, &change, query, id); err != nil {
		return nil, err
	}
	return &change, nil
}

func (r *DestinationChangeRepository) List(ctx context.Context, filter domain.DestinationChangeFilter) ([]domain.DestinationChangeRequest, error) {
	var (
		parts []string
		args  []any
		idx   = 1
	)

	if filter.DestinationID != nil {
		parts = append(parts, fmt.Sprintf("destination_id = $%d", idx))
		args = append(args, *filter.DestinationID)
		idx++
	}
	if filter.SubmittedBy != nil {
		parts = append(parts, fmt.Sprintf("submitted_by = $%d", idx))
		args = append(args, *filter.SubmittedBy)
		idx++
	}
	if len(filter.Statuses) > 0 {
		in := make([]string, 0, len(filter.Statuses))
		for _, status := range filter.Statuses {
			in = append(in, fmt.Sprintf("$%d", idx))
			args = append(args, status)
			idx++
		}
		parts = append(parts, fmt.Sprintf("status IN (%s)", strings.Join(in, ",")))
	}

	where := ""
	if len(parts) > 0 {
		where = "WHERE " + strings.Join(parts, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, destination_id, action, payload, hero_image_temp_key, status,
		       draft_version, submitted_by, reviewed_by, submitted_at, reviewed_at,
		       review_message, published_version, created_at, updated_at
		FROM destination_change_request
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, idx, idx+1)

	args = append(args, filter.Limit, filter.Offset)

	changes := make([]domain.DestinationChangeRequest, 0)
	if err := r.db.SelectContext(ctx, &changes, query, args...); err != nil {
		return nil, err
	}
	return changes, nil
}

func (r *DestinationChangeRepository) SetStatus(ctx context.Context, id uuid.UUID, status domain.DestinationChangeStatus, reviewerID *uuid.UUID, reviewMessage *string, publishedVersion *int64) (*domain.DestinationChangeRequest, error) {
	const query = `
		UPDATE destination_change_request
		SET status = $2,
		    reviewed_by = $3,
		    review_message = $4,
		    reviewed_at = CASE WHEN $2 IN ('approved','rejected') THEN NOW() ELSE reviewed_at END,
		    published_version = $5,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, destination_id, action, payload, hero_image_temp_key, status,
		          draft_version, submitted_by, reviewed_by, submitted_at, reviewed_at,
		          review_message, published_version, created_at, updated_at
	`
	var change domain.DestinationChangeRequest
	if err := r.db.GetContext(ctx, &change, query, id, status, nullableUUID(reviewerID), nullString(reviewMessage), publishedVersion); err != nil {
		return nil, err
	}
	return &change, nil
}

func (r *DestinationChangeRepository) DeleteByID(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM destination_change_request WHERE id = $1`, id)
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

func nullableUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

var _ ports.DestinationChangeRepository = (*DestinationChangeRepository)(nil)

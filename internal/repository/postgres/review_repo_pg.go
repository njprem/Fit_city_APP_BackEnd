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

type ReviewRepository struct {
	db *sqlx.DB
}

func NewReviewRepo(db *sqlx.DB) *ReviewRepository {
	return &ReviewRepository{db: db}
}

func (r *ReviewRepository) Create(ctx context.Context, review *domain.Review) (*domain.Review, error) {
	const query = `
		INSERT INTO review (user_id, destination_id, rating, title, content)
		VALUES (:user_id, :destination_id, :rating, :title, :content)
		RETURNING id, user_id, destination_id, rating, title, content, created_at, updated_at, deleted_at, deleted_by
	`
	args := map[string]any{
		"user_id":        review.UserID,
		"destination_id": review.DestinationID,
		"rating":         review.Rating,
		"title":          nullString(review.Title),
		"content":        nullString(review.Content),
	}

	rows, err := r.db.NamedQueryContext(ctx, query, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		var stored domain.Review
		if err := rows.StructScan(&stored); err != nil {
			return nil, err
		}
		return &stored, nil
	}
	return nil, sql.ErrNoRows
}

func (r *ReviewRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Review, error) {
	const query = `
		SELECT
			r.id,
			r.user_id,
			r.destination_id,
			r.rating,
			r.title,
			r.content,
			r.created_at,
			r.updated_at,
			r.deleted_at,
			r.deleted_by,
			u.full_name AS reviewer_name,
			u.username AS reviewer_username,
			u.user_image_url AS reviewer_avatar_url,
			u.email AS reviewer_email
		FROM review r
		JOIN user_account u ON u.id = r.user_id
		WHERE r.id = $1
	`

	var review domain.Review
	if err := r.db.GetContext(ctx, &review, query, id); err != nil {
		return nil, err
	}
	return &review, nil
}

func (r *ReviewRepository) ListByDestination(ctx context.Context, destinationID uuid.UUID, filter domain.ReviewListFilter) ([]domain.Review, error) {
	clauses := []string{"r.destination_id = $1", "r.deleted_at IS NULL"}
	args := []any{destinationID}
	idx := 2

	if filter.Rating != nil {
		clauses = append(clauses, fmt.Sprintf("r.rating = $%d", idx))
		args = append(args, *filter.Rating)
		idx++
	}
	if filter.MinRating != nil {
		clauses = append(clauses, fmt.Sprintf("r.rating >= $%d", idx))
		args = append(args, *filter.MinRating)
		idx++
	}
	if filter.MaxRating != nil {
		clauses = append(clauses, fmt.Sprintf("r.rating <= $%d", idx))
		args = append(args, *filter.MaxRating)
		idx++
	}
	if filter.PostedAfter != nil {
		clauses = append(clauses, fmt.Sprintf("r.created_at >= $%d", idx))
		args = append(args, *filter.PostedAfter)
		idx++
	}
	if filter.PostedBefore != nil {
		clauses = append(clauses, fmt.Sprintf("r.created_at <= $%d", idx))
		args = append(args, *filter.PostedBefore)
		idx++
	}

	where := "WHERE " + strings.Join(clauses, " AND ")

	sortCol := "r.created_at"
	switch filter.SortField {
	case domain.ReviewSortRating:
		sortCol = "r.rating"
	}
	order := "DESC"
	if filter.SortOrder == domain.SortOrderAsc {
		order = "ASC"
	}

	args = append(args, filter.Limit, filter.Offset)

	query := fmt.Sprintf(`
		SELECT
			r.id,
			r.user_id,
			r.destination_id,
			r.rating,
			r.title,
			r.content,
			r.created_at,
			r.updated_at,
			r.deleted_at,
			r.deleted_by,
			u.full_name AS reviewer_name,
			u.username AS reviewer_username,
			u.user_image_url AS reviewer_avatar_url,
			u.email AS reviewer_email
		FROM review r
		JOIN user_account u ON u.id = r.user_id
		%s
		ORDER BY %s %s, r.id DESC
		LIMIT $%d OFFSET $%d
	`, where, sortCol, order, idx, idx+1)

	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []domain.Review
	for rows.Next() {
		var review domain.Review
		if err := rows.StructScan(&review); err != nil {
			return nil, err
		}
		reviews = append(reviews, review)
	}
	return reviews, rows.Err()
}

func (r *ReviewRepository) AggregateByDestination(ctx context.Context, destinationID uuid.UUID, filter domain.ReviewAggregateFilter) (*domain.ReviewAggregate, error) {
	clauses := []string{"r.destination_id = $1", "r.deleted_at IS NULL"}
	args := []any{destinationID}
	idx := 2

	if filter.Rating != nil {
		clauses = append(clauses, fmt.Sprintf("r.rating = $%d", idx))
		args = append(args, *filter.Rating)
		idx++
	}
	if filter.MinRating != nil {
		clauses = append(clauses, fmt.Sprintf("r.rating >= $%d", idx))
		args = append(args, *filter.MinRating)
		idx++
	}
	if filter.MaxRating != nil {
		clauses = append(clauses, fmt.Sprintf("r.rating <= $%d", idx))
		args = append(args, *filter.MaxRating)
		idx++
	}
	if filter.PostedAfter != nil {
		clauses = append(clauses, fmt.Sprintf("r.created_at >= $%d", idx))
		args = append(args, *filter.PostedAfter)
		idx++
	}
	if filter.PostedBefore != nil {
		clauses = append(clauses, fmt.Sprintf("r.created_at <= $%d", idx))
		args = append(args, *filter.PostedBefore)
		idx++
	}

	where := "WHERE " + strings.Join(clauses, " AND ")

	query := fmt.Sprintf(`
		SELECT
			COUNT(*)::int AS total_reviews,
			COALESCE(AVG(r.rating)::float8, 0) AS average_rating,
			COUNT(*) FILTER (WHERE r.rating = 0)::int AS rating_0,
			COUNT(*) FILTER (WHERE r.rating = 1)::int AS rating_1,
			COUNT(*) FILTER (WHERE r.rating = 2)::int AS rating_2,
			COUNT(*) FILTER (WHERE r.rating = 3)::int AS rating_3,
			COUNT(*) FILTER (WHERE r.rating = 4)::int AS rating_4,
			COUNT(*) FILTER (WHERE r.rating = 5)::int AS rating_5
		FROM review r
		%s
	`, where)

	var row struct {
		Total   int     `db:"total_reviews"`
		Average float64 `db:"average_rating"`
		Rating0 int     `db:"rating_0"`
		Rating1 int     `db:"rating_1"`
		Rating2 int     `db:"rating_2"`
		Rating3 int     `db:"rating_3"`
		Rating4 int     `db:"rating_4"`
		Rating5 int     `db:"rating_5"`
	}

	if err := r.db.GetContext(ctx, &row, query, args...); err != nil {
		return nil, err
	}

	counts := map[int]int{
		0: row.Rating0,
		1: row.Rating1,
		2: row.Rating2,
		3: row.Rating3,
		4: row.Rating4,
		5: row.Rating5,
	}

	return &domain.ReviewAggregate{
		DestinationID: destinationID,
		AverageRating: row.Average,
		TotalReviews:  row.Total,
		RatingCounts:  counts,
	}, nil
}

func (r *ReviewRepository) SoftDelete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error {
	const query = `
		UPDATE review
		SET deleted_at = NOW(), deleted_by = $2
		WHERE id = $1 AND deleted_at IS NULL
	`
	result, err := r.db.ExecContext(ctx, query, id, deletedBy)
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

func (r *ReviewRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const query = `DELETE FROM review WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

var _ ports.ReviewRepository = (*ReviewRepository)(nil)

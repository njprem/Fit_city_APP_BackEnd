package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

type DestinationImportRepository struct {
	db *sqlx.DB
}

func NewDestinationImportRepo(db *sqlx.DB) *DestinationImportRepository {
	return &DestinationImportRepository{db: db}
}

func (r *DestinationImportRepository) CreateJob(ctx context.Context, job *domain.DestinationImportJob) (*domain.DestinationImportJob, error) {
	const query = `
		INSERT INTO destination_import_job (
			id, uploaded_by, status, dry_run, file_key, error_csv_key, notes,
			total_rows, processed_rows, rows_failed, changes_created,
			submitted_at, completed_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11,
			$12, $13, NOW(), NOW()
		)
		RETURNING id, uploaded_by, status, dry_run, file_key, error_csv_key, notes,
		          total_rows, processed_rows, rows_failed, changes_created,
		          submitted_at, completed_at, created_at, updated_at
	`

	var inserted domain.DestinationImportJob
	if err := r.db.GetContext(ctx, &inserted, query,
		job.ID,
		job.UploadedBy,
		job.Status,
		job.DryRun,
		job.FileKey,
		nullStringPtr(job.ErrorCSVKey),
		nullStringPtr(job.Notes),
		job.TotalRows,
		job.ProcessedRows,
		job.RowsFailed,
		job.ChangesCreated,
		job.SubmittedAt,
		nullTimePtr(job.CompletedAt),
	); err != nil {
		return nil, err
	}
	return &inserted, nil
}

func (r *DestinationImportRepository) UpdateJob(ctx context.Context, job *domain.DestinationImportJob) (*domain.DestinationImportJob, error) {
	const query = `
		UPDATE destination_import_job
		SET status = $2,
		    dry_run = $3,
		    file_key = $4,
		    error_csv_key = $5,
		    notes = $6,
		    total_rows = $7,
		    processed_rows = $8,
		    rows_failed = $9,
		    changes_created = $10,
		    submitted_at = $11,
		    completed_at = $12,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, uploaded_by, status, dry_run, file_key, error_csv_key, notes,
		          total_rows, processed_rows, rows_failed, changes_created,
		          submitted_at, completed_at, created_at, updated_at
	`

	var updated domain.DestinationImportJob
	if err := r.db.GetContext(ctx, &updated, query,
		job.ID,
		job.Status,
		job.DryRun,
		job.FileKey,
		nullStringPtr(job.ErrorCSVKey),
		nullStringPtr(job.Notes),
		job.TotalRows,
		job.ProcessedRows,
		job.RowsFailed,
		job.ChangesCreated,
		job.SubmittedAt,
		nullTimePtr(job.CompletedAt),
	); err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *DestinationImportRepository) FindJobByID(ctx context.Context, id uuid.UUID) (*domain.DestinationImportJob, error) {
	const query = `
		SELECT id, uploaded_by, status, dry_run, file_key, error_csv_key, notes,
		       total_rows, processed_rows, rows_failed, changes_created,
		       submitted_at, completed_at, created_at, updated_at
		FROM destination_import_job
		WHERE id = $1
	`

	var job domain.DestinationImportJob
	if err := r.db.GetContext(ctx, &job, query, id); err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *DestinationImportRepository) InsertRow(ctx context.Context, row *domain.DestinationImportRow) (*domain.DestinationImportRow, error) {
	const query = `
		INSERT INTO destination_import_row (
			job_id, row_number, status, action, change_id, error, payload
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
		RETURNING id, job_id, row_number, status, action, change_id, error, payload, created_at
	`

	var inserted domain.DestinationImportRow
	if err := r.db.GetContext(ctx, &inserted, query,
		row.JobID,
		row.RowNumber,
		row.Status,
		row.Action,
		uuidPtrOrNil(row.ChangeID),
		nullStringPtr(row.ErrorMessage),
		row.Payload,
	); err != nil {
		return nil, err
	}
	return &inserted, nil
}

func (r *DestinationImportRepository) ListRowsByJob(ctx context.Context, jobID uuid.UUID) ([]domain.DestinationImportRow, error) {
	const query = `
		SELECT id, job_id, row_number, status, action, change_id, error, payload, created_at
		FROM destination_import_row
		WHERE job_id = $1
		ORDER BY row_number ASC
	`
	rows := make([]domain.DestinationImportRow, 0)
	if err := r.db.SelectContext(ctx, &rows, query, jobID); err != nil {
		return nil, err
	}
	return rows, nil
}

func nullStringPtr(ptr *string) sql.NullString {
	if ptr == nil {
		return sql.NullString{Valid: false}
	}
	if *ptr == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *ptr, Valid: true}
}

func nullTimePtr(ptr *time.Time) sql.NullTime {
	if ptr == nil {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Time: *ptr, Valid: true}
}

func uuidPtrOrNil(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

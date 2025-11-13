package domain

import (
	"time"

	"github.com/google/uuid"
)

type DestinationImportStatus string

const (
	DestinationImportStatusQueued     DestinationImportStatus = "queued"
	DestinationImportStatusProcessing DestinationImportStatus = "processing"
	DestinationImportStatusCompleted  DestinationImportStatus = "completed"
	DestinationImportStatusFailed     DestinationImportStatus = "failed"
)

type DestinationImportRowStatus string

const (
	DestinationImportRowStatusPendingReview DestinationImportRowStatus = "pending_review"
	DestinationImportRowStatusSkipped       DestinationImportRowStatus = "skipped"
	DestinationImportRowStatusFailed        DestinationImportRowStatus = "failed"
)

type DestinationImportJob struct {
	ID             uuid.UUID               `db:"id" json:"id"`
	UploadedBy     uuid.UUID               `db:"uploaded_by" json:"uploaded_by"`
	Status         DestinationImportStatus `db:"status" json:"status"`
	DryRun         bool                    `db:"dry_run" json:"dry_run"`
	FileKey        string                  `db:"file_key" json:"file_key"`
	ErrorCSVKey    *string                 `db:"error_csv_key" json:"error_csv_key,omitempty"`
	Notes          *string                 `db:"notes" json:"notes,omitempty"`
	TotalRows      int                     `db:"total_rows" json:"total_rows"`
	ProcessedRows  int                     `db:"processed_rows" json:"processed_rows"`
	RowsFailed     int                     `db:"rows_failed" json:"rows_failed"`
	ChangesCreated int                     `db:"changes_created" json:"changes_created"`
	PendingIDs     []uuid.UUID             `db:"-" json:"pending_change_ids,omitempty"`
	SubmittedAt    time.Time               `db:"submitted_at" json:"submitted_at"`
	CompletedAt    *time.Time              `db:"completed_at" json:"completed_at,omitempty"`
	CreatedAt      time.Time               `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time               `db:"updated_at" json:"updated_at"`
	Rows           []DestinationImportRow  `db:"-" json:"rows,omitempty"`
}

type DestinationImportRow struct {
	ID           uuid.UUID                  `db:"id" json:"id"`
	JobID        uuid.UUID                  `db:"job_id" json:"job_id"`
	RowNumber    int                        `db:"row_number" json:"row_number"`
	Status       DestinationImportRowStatus `db:"status" json:"status"`
	Action       DestinationChangeAction    `db:"action" json:"action"`
	ChangeID     *uuid.UUID                 `db:"change_id" json:"change_id,omitempty"`
	ErrorMessage *string                    `db:"error" json:"error,omitempty"`
	Payload      DestinationChangeFields    `db:"payload" json:"payload"`
	CreatedAt    time.Time                  `db:"created_at" json:"created_at"`
}

BEGIN;

CREATE TABLE IF NOT EXISTS destination_import_job (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    uploaded_by UUID NOT NULL REFERENCES user_account(id),
    status TEXT NOT NULL DEFAULT 'queued',
    dry_run BOOLEAN NOT NULL DEFAULT FALSE,
    file_key TEXT NOT NULL,
    error_csv_key TEXT,
    notes TEXT,
    total_rows INT NOT NULL DEFAULT 0,
    processed_rows INT NOT NULL DEFAULT 0,
    rows_failed INT NOT NULL DEFAULT 0,
    changes_created INT NOT NULL DEFAULT 0,
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT destination_import_job_status_check CHECK (status IN ('queued','processing','completed','failed'))
);

CREATE INDEX IF NOT EXISTS idx_destination_import_job_status
    ON destination_import_job(status);

CREATE TABLE IF NOT EXISTS destination_import_row (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    job_id UUID NOT NULL REFERENCES destination_import_job(id) ON DELETE CASCADE,
    row_number INT NOT NULL,
    status TEXT NOT NULL,
    action TEXT NOT NULL,
    change_id UUID,
    error TEXT,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT destination_import_row_status_check CHECK (status IN ('pending_review','skipped','failed'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_destination_import_row_job_row
    ON destination_import_row(job_id, row_number);

CREATE INDEX IF NOT EXISTS idx_destination_import_row_job
    ON destination_import_row(job_id);

COMMIT;


BEGIN;

ALTER TABLE travel_destination
    ADD COLUMN IF NOT EXISTS slug TEXT,
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'draft',
    ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS updated_by UUID REFERENCES user_account(id),
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_travel_destination_status
    ON travel_destination(status);

CREATE INDEX IF NOT EXISTS idx_travel_destination_updated_at
    ON travel_destination(updated_at DESC);

UPDATE travel_destination
SET status = 'published'
WHERE status IS NULL OR status = '';

ALTER TABLE travel_destination
    ADD CONSTRAINT travel_destination_status_check CHECK (status IN ('draft', 'published', 'archived'));

CREATE UNIQUE INDEX IF NOT EXISTS travel_destination_slug_idx
    ON travel_destination(slug) WHERE slug IS NOT NULL;

CREATE TABLE IF NOT EXISTS destination_change_request (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    destination_id UUID REFERENCES travel_destination(id),
    action TEXT NOT NULL,
    payload JSONB NOT NULL,
    hero_image_temp_key TEXT,
    status TEXT NOT NULL DEFAULT 'draft',
    draft_version INT NOT NULL DEFAULT 1,
    submitted_by UUID NOT NULL REFERENCES user_account(id),
    reviewed_by UUID REFERENCES user_account(id),
    submitted_at TIMESTAMPTZ,
    reviewed_at TIMESTAMPTZ,
    review_message TEXT,
    published_version BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT destination_change_request_action_check CHECK (action IN ('create', 'update', 'delete')),
    CONSTRAINT destination_change_request_status_check CHECK (status IN ('draft', 'pending_review', 'approved', 'rejected'))
);

CREATE INDEX IF NOT EXISTS destination_change_request_status_idx
    ON destination_change_request(status);

CREATE INDEX IF NOT EXISTS destination_change_request_destination_idx
    ON destination_change_request(destination_id);

CREATE TABLE IF NOT EXISTS destination_version (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    destination_id UUID NOT NULL REFERENCES travel_destination(id),
    change_request_id UUID REFERENCES destination_change_request(id),
    version BIGINT NOT NULL,
    snapshot JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL REFERENCES user_account(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS destination_version_unique
    ON destination_version(destination_id, version);

CREATE TABLE IF NOT EXISTS travel_destination_audit (
    id BIGSERIAL PRIMARY KEY,
    destination_id UUID NOT NULL REFERENCES travel_destination(id) ON DELETE CASCADE,
    actor_id UUID NOT NULL REFERENCES user_account(id),
    operation TEXT NOT NULL,
    result TEXT NOT NULL DEFAULT 'success',
    changes JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_travel_destination_audit_destination
    ON travel_destination_audit(destination_id);

CREATE INDEX IF NOT EXISTS idx_travel_destination_audit_created_at
    ON travel_destination_audit(created_at DESC);

COMMIT;

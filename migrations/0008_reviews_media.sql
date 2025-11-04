-- Adjust review rating constraint to allow zero and add soft delete metadata
ALTER TABLE review
    DROP CONSTRAINT IF EXISTS review_rating_check;

ALTER TABLE review
    ADD CONSTRAINT review_rating_check CHECK (rating BETWEEN 0 AND 5);

ALTER TABLE review
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_by UUID REFERENCES user_account(id);

CREATE INDEX IF NOT EXISTS review_active_destination_idx
    ON review(destination_id)
    WHERE deleted_at IS NULL;

-- Store media attachments for reviews
CREATE TABLE IF NOT EXISTS review_media (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    review_id UUID NOT NULL REFERENCES review(id) ON DELETE CASCADE,
    object_key TEXT NOT NULL,
    url TEXT NOT NULL,
    ordering INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS review_media_review_id_idx
    ON review_media(review_id);

-- Ensure only one active (non-soft-deleted) review per user/destination pair.
-- Allows users to submit a new review once the previous one is soft deleted.

DROP INDEX IF EXISTS review_unique_user_destination_idx;

CREATE UNIQUE INDEX IF NOT EXISTS review_user_destination_active_idx
    ON review(user_id, destination_id)
    WHERE deleted_at IS NULL;

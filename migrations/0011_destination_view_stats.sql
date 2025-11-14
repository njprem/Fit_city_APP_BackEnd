BEGIN;

CREATE TABLE IF NOT EXISTS destination_view_stats (
    destination_id UUID NOT NULL REFERENCES travel_destination(id),
    range_key TEXT NOT NULL,
    bucket_start TIMESTAMPTZ NOT NULL,
    bucket_end TIMESTAMPTZ NOT NULL,
    total_views BIGINT NOT NULL DEFAULT 0,
    unique_users INT NOT NULL DEFAULT 0,
    unique_ips INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (destination_id, range_key)
);

CREATE INDEX IF NOT EXISTS idx_destination_view_stats_destination
    ON destination_view_stats(destination_id);

CREATE INDEX IF NOT EXISTS idx_destination_view_stats_bucket_end
    ON destination_view_stats(bucket_end);

CREATE TABLE IF NOT EXISTS destination_view_rollup_checkpoint (
    id INT PRIMARY KEY DEFAULT 1,
    last_bucket_end TIMESTAMPTZ NOT NULL
);

INSERT INTO destination_view_rollup_checkpoint(id, last_bucket_end)
    VALUES (1, '1970-01-01 00:00:00+00')
    ON CONFLICT (id) DO NOTHING;

COMMIT;

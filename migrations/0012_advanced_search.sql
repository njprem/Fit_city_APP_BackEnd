CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_destination_name_trgm
    ON travel_destination
    USING GIN (name gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_travel_destination_description_trgm
    ON travel_destination
    USING GIN (description gin_trgm_ops);
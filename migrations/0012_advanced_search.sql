CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_destination_name_trgm
    ON travel_destination
    USING GIN (name gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_travel_destination_description_trgm
    ON travel_destination
    USING GIN (description gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx__destination_city
    ON travel_destination (city);

CREATE INDEX IF NOT EXISTS idx_destination_country
    ON travel_destination (country);

CREATE INDEX IF NOT EXISTS idx_destination_city_trgm
    ON travel_destination
    USING GIN (city gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_destination_country_trgm
    ON travel_destination
    USING GIN (country gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx__destination_lat_lng
    ON travel_destination (latitude, longitude);

COMMIT;
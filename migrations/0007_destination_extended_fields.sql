BEGIN;

ALTER TABLE travel_destination
    ADD COLUMN IF NOT EXISTS contact TEXT,
    ADD COLUMN IF NOT EXISTS opening_time TEXT,
    ADD COLUMN IF NOT EXISTS closing_time TEXT,
    ADD COLUMN IF NOT EXISTS gallery JSONB;

UPDATE travel_destination
SET gallery = '[]'::jsonb
WHERE gallery IS NULL;

COMMIT;

CREATE TABLE IF NOT EXISTS travel_destination (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    city TEXT,
    country TEXT,
    description TEXT,
    category TEXT,
    latitude DOUBLE PRECISION,
    longitude DOUBLE PRECISION,
    hero_image_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
